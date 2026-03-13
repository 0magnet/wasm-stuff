//go:build js && wasm

package main

import (
	"strconv"
	"syscall/js"
	"unsafe"

	"github.com/go-gl/mathgl/mgl32"
)

var fragShaderCode = `
	precision mediump float;
	uniform vec3 uBaseColor;
	uniform vec3 uTopColor;
	uniform vec3 uMidColor;
	uniform float uMinZ;
	uniform float uMaxZ;
	uniform float uMinX;
	uniform float uMaxX;
	uniform int uGradientMode;
	varying vec3 vPosition;
	void main(void) {
		vec3 color;
		if (uGradientMode == 1) {
			// Lorenz dual-lobe: x-based three-color gradient
			// left lobe (base) -> origin (mid) -> right lobe (top)
			float xRange = max(uMaxX - uMinX, 0.001);
			float tx = clamp((vPosition.x - uMinX) / xRange, 0.0, 1.0);
			if (tx < 0.5) {
				color = mix(uBaseColor, uMidColor, tx * 2.0);
			} else {
				color = mix(uMidColor, uTopColor, (tx - 0.5) * 2.0);
			}
		} else {
			// Standard: z-based two-color gradient
			float zRange = max(uMaxZ - uMinZ, 0.001);
			float tz = clamp((vPosition.z - uMinZ) / zRange, 0.0, 1.0);
			color = mix(uBaseColor, uTopColor, tz);
		}
		gl_FragColor = vec4(color, 1.0);
	}
`

var vertShaderCode = `
	attribute vec3 position;
	uniform mat4 Pmatrix;
	uniform mat4 Vmatrix;
	uniform mat4 Mmatrix;
	varying vec3 vPosition;
	void main(void) {
		gl_Position = Pmatrix * Vmatrix * Mmatrix * vec4(position, 1.0);
		gl_PointSize = 2.0;
		vPosition = position;
	}
`

func setupShaders() {
	vertShader := gl.Call("createShader", glTypes.VertexShader)
	gl.Call("shaderSource", vertShader, vertShaderCode)
	gl.Call("compileShader", vertShader)

	fragShader := gl.Call("createShader", glTypes.FragmentShader)
	gl.Call("shaderSource", fragShader, fragShaderCode)
	gl.Call("compileShader", fragShader)

	gl.Call("attachShader", shaderProgram, vertShader)
	gl.Call("attachShader", shaderProgram, fragShader)
	gl.Call("linkProgram", shaderProgram)

	position := gl.Call("getAttribLocation", shaderProgram, "position")
	gl.Call("vertexAttribPointer", position, 3, glTypes.Float, false, 0, 0)
	gl.Call("enableVertexAttribArray", position)
	gl.Call("useProgram", shaderProgram)

	uBaseColorLoc = gl.Call("getUniformLocation", shaderProgram, "uBaseColor")
	uTopColorLoc = gl.Call("getUniformLocation", shaderProgram, "uTopColor")
	uMidColorLoc = gl.Call("getUniformLocation", shaderProgram, "uMidColor")
	uMinZLoc = gl.Call("getUniformLocation", shaderProgram, "uMinZ")
	uMaxZLoc = gl.Call("getUniformLocation", shaderProgram, "uMaxZ")
	uMinXLoc = gl.Call("getUniformLocation", shaderProgram, "uMinX")
	uMaxXLoc = gl.Call("getUniformLocation", shaderProgram, "uMaxX")
	uGradientModeLoc = gl.Call("getUniformLocation", shaderProgram, "uGradientMode")
	gl.Call("uniform3f", uBaseColorLoc, baseColor[0], baseColor[1], baseColor[2])
	gl.Call("uniform3f", uTopColorLoc, topColor[0], topColor[1], topColor[2])
	gl.Call("uniform3f", uMidColorLoc, midColor[0], midColor[1], midColor[2])
	gl.Call("uniform1f", uMinZLoc, float64(-1))
	gl.Call("uniform1f", uMaxZLoc, float64(1))
	gl.Call("uniform1f", uMinXLoc, float64(-1))
	gl.Call("uniform1f", uMaxXLoc, float64(1))
	gl.Call("uniform1i", uGradientModeLoc, 0)
	shadersReady = true

	gl.Call("clearColor", 0, 0, 0, 0)
	gl.Call("clearDepth", 1.0)
	gl.Call("viewport", 0, 0, width, height)
	gl.Call("depthFunc", glTypes.LEqual)
}

func setupMatrices() {
	projMatrix := mgl32.Perspective(mgl32.DegToRad(45.0), float32(width)/float32(height), 1, 100.0)
	projMatrixBuffer := (*[16]float32)(unsafe.Pointer(&projMatrix))
	typedProjMatrixBuffer := SliceToTypedArray([]float32((*projMatrixBuffer)[:]))
	gl.Call("uniformMatrix4fv", gl.Call("getUniformLocation", shaderProgram, "Pmatrix"), false, typedProjMatrixBuffer)

	movMatrix = mgl32.Ident4()
	updateViewMatrix()
	updateModelMatrix()
}

func updateViewMatrix() {
	cameraPosition := mgl32.Vec3{0.0, 0.0, defaultCameraDist}
	center := mgl32.Vec3{0.0, 0.0, 0.0}
	if len(attractorVertices) > 0 {
		for i := 0; i < len(attractorVertices); i += 3 {
			center[0] += attractorVertices[i]
			center[1] += attractorVertices[i+1]
			center[2] += attractorVertices[i+2]
		}
		center = center.Mul(1.0 / float32(len(attractorVertices)/3))
	}
	cameraDirection := center.Sub(cameraPosition).Normalize()
	upVector := mgl32.Vec3{0.0, 1.0, 0.0}
	rightVector := cameraDirection.Cross(upVector).Normalize()
	newUpVector := rightVector.Cross(cameraDirection)
	viewMatrix := mgl32.LookAtV(cameraPosition, center, newUpVector)
	viewMatrixBuffer := (*[16]float32)(unsafe.Pointer(&viewMatrix))
	typedViewMatrixBuffer := SliceToTypedArray([]float32((*viewMatrixBuffer)[:]))
	gl.Call("uniformMatrix4fv", gl.Call("getUniformLocation", shaderProgram, "Vmatrix"), false, typedViewMatrixBuffer)
}

func updateModelMatrix() {
	modelMatrixBuffer := (*[16]float32)(unsafe.Pointer(&movMatrix))
	typedModelMatrixBuffer := SliceToTypedArray([]float32((*modelMatrixBuffer)[:]))
	gl.Call("uniformMatrix4fv", gl.Call("getUniformLocation", shaderProgram, "Mmatrix"), false, typedModelMatrixBuffer)
}

func autoFitCamera() {
	if len(attractorVertices) < 3 {
		return
	}
	maxAbs := float32(0)
	for i := 0; i < len(attractorVertices); i++ {
		v := attractorVertices[i]
		if v < 0 {
			v = -v
		}
		if v > maxAbs {
			maxAbs = v
		}
	}
	// Set camera distance to ~3x the max extent so the whole thing is visible
	dist := maxAbs * 3.0
	if dist < 5 {
		dist = 5
	}
	if dist > 300 {
		dist = 300
	}
	initCameraDist = dist
	defaultCameraDist = dist
	cameraControl.Set("value", "0")
	sliderZoom.Set("textContent", "0")
	updateViewMatrix()
}

func generateForMode(mode string) {
	if shadersReady {
		if mode == "lorenz" {
			gl.Call("uniform1i", uGradientModeLoc, 1)
		} else {
			gl.Call("uniform1i", uGradientModeLoc, 0)
		}
	}
	switch mode {
	case "lorenz":
		generateLorenz()
	case "rossler":
		generateRossler()
	case "chua":
		generateChua()
	case "aizawa":
		generateAizawa()
	case "sprott":
		generateSprott()
	case "lissajou":
		generateLissajou()
	case "thomas":
		generateThomas()
	case "halvorsen":
		generateHalvorsen()
	case "chen":
		generateChen()
	case "dadras":
		generateDadras()
	case "rabinovich":
		generateRabinovich()
	case "burkeshaw":
		generateBurkeShaw()
	case "cube":
		generateCube()
	case "nestedcube":
		generateNestedCube()
	case "sphere":
		generateSphere()
	case "torus":
		generateTorus()
	case "magnetosphere":
		generateMagnetosphere()
	default:
		generateRossler()
	}
}

func renderLoop(this js.Value, args []js.Value) interface{} {
	now := float32(args[0].Float())
	tdiff := now - tmark
	tmark = now
	totalelapsed += tdiff

	gl.Call("enable", glTypes.DepthTest)
	gl.Call("clear", glTypes.ColorBufferBit)
	gl.Call("clear", glTypes.DepthBufferBit)

	generateForMode(selectedMode)

	// Read slider values
	zoomVal := float32(js.Global().Get("parseFloat").Invoke(cameraControl.Get("value")).Float())
	rotationX = float32(js.Global().Get("parseFloat").Invoke(rotationControlsX.Get("value")).Float())
	rotationY = float32(js.Global().Get("parseFloat").Invoke(rotationControlsY.Get("value")).Float())
	rotationZ = float32(js.Global().Get("parseFloat").Invoke(rotationControlsZ.Get("value")).Float())
	sliderZoom.Set("textContent", strconv.FormatFloat(float64(zoomVal), 'f', 0, 64))
	sliderX.Set("textContent", strconv.FormatFloat(float64(rotationX), 'f', 1, 64))
	sliderY.Set("textContent", strconv.FormatFloat(float64(rotationY), 'f', 1, 64))
	sliderZ.Set("textContent", strconv.FormatFloat(float64(rotationZ), 'f', 1, 64))

	// Zoom slider directly controls camera distance (absolute position)
	newDist := initCameraDist - zoomVal
	if newDist != defaultCameraDist {
		defaultCameraDist = newDist
		updateViewMatrix()
	}
	if rotationX != 0 {
		rotationX1 = rotationX/20 + tdiff*0.00000001
		movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DX(rotationX1))
	}
	if rotationY != 0 {
		rotationY1 = rotationY/20 + tdiff*0.00000001
		movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DY(rotationY1))
	}
	if rotationZ != 0 {
		rotationZ1 = rotationZ/20 + tdiff*0.00000001
		movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DZ(rotationZ1))
	}

	// Apply mouse/touch drag rotation
	if dragRotX != 0 || dragRotY != 0 {
		movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DX(dragRotX))
		movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DY(dragRotY))
		dragRotX = 0
		dragRotY = 0
	}

	// Auto-rotation
	if autoRotate {
		movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DY(autoRotateSpeed))
	}

	updateModelMatrix()

	js.Global().Call("requestAnimationFrame", renderFrame)
	return nil
}
