//go:build js && wasm

package attractor

import (
	"syscall/js"

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
	uniform float uMinY;
	uniform float uMaxY;
	uniform int uGradientMode;
	uniform int uGradientReverse;
	varying vec3 vPosition;
	varying float vTrailT;

	vec3 hsv2rgb(vec3 c) {
		vec4 K = vec4(1.0, 2.0/3.0, 1.0/3.0, 3.0);
		vec3 p = abs(fract(c.xxx + K.xyz) * 6.0 - K.www);
		return c.z * mix(K.xxx, clamp(p - K.xxx, 0.0, 1.0), c.y);
	}

	void main(void) {
		float t;
		vec3 color;
		// Mode 0: Z two-color, 1: X three-color, 2: Y two-color, 3: X two-color
		// Mode 4: Trail rainbow, 5: Trail two-color, 6: Trail three-color
		// Mode 7: Z rainbow, 8: X rainbow, 9: Y rainbow
		if (uGradientMode == 1) {
			float xRange = max(uMaxX - uMinX, 0.001);
			t = clamp((vPosition.x - uMinX) / xRange, 0.0, 1.0);
			if (uGradientReverse == 1) t = 1.0 - t;
			if (t < 0.5) {
				color = mix(uBaseColor, uMidColor, t * 2.0);
			} else {
				color = mix(uMidColor, uTopColor, (t - 0.5) * 2.0);
			}
		} else if (uGradientMode == 2) {
			float yRange = max(uMaxY - uMinY, 0.001);
			t = clamp((vPosition.y - uMinY) / yRange, 0.0, 1.0);
			if (uGradientReverse == 1) t = 1.0 - t;
			color = mix(uBaseColor, uTopColor, t);
		} else if (uGradientMode == 3) {
			float xRange = max(uMaxX - uMinX, 0.001);
			t = clamp((vPosition.x - uMinX) / xRange, 0.0, 1.0);
			if (uGradientReverse == 1) t = 1.0 - t;
			color = mix(uBaseColor, uTopColor, t);
		} else if (uGradientMode == 4) {
			t = vTrailT;
			if (uGradientReverse == 1) t = 1.0 - t;
			color = hsv2rgb(vec3(t, 1.0, 1.0));
		} else if (uGradientMode == 5) {
			t = vTrailT;
			if (uGradientReverse == 1) t = 1.0 - t;
			color = mix(uBaseColor, uTopColor, t);
		} else if (uGradientMode == 6) {
			t = vTrailT;
			if (uGradientReverse == 1) t = 1.0 - t;
			if (t < 0.5) {
				color = mix(uBaseColor, uMidColor, t * 2.0);
			} else {
				color = mix(uMidColor, uTopColor, (t - 0.5) * 2.0);
			}
		} else if (uGradientMode == 7) {
			float zRange = max(uMaxZ - uMinZ, 0.001);
			t = clamp((vPosition.z - uMinZ) / zRange, 0.0, 1.0);
			if (uGradientReverse == 1) t = 1.0 - t;
			color = hsv2rgb(vec3(t, 1.0, 1.0));
		} else if (uGradientMode == 8) {
			float xRange = max(uMaxX - uMinX, 0.001);
			t = clamp((vPosition.x - uMinX) / xRange, 0.0, 1.0);
			if (uGradientReverse == 1) t = 1.0 - t;
			color = hsv2rgb(vec3(t, 1.0, 1.0));
		} else if (uGradientMode == 9) {
			float yRange = max(uMaxY - uMinY, 0.001);
			t = clamp((vPosition.y - uMinY) / yRange, 0.0, 1.0);
			if (uGradientReverse == 1) t = 1.0 - t;
			color = hsv2rgb(vec3(t, 1.0, 1.0));
		} else {
			float zRange = max(uMaxZ - uMinZ, 0.001);
			t = clamp((vPosition.z - uMinZ) / zRange, 0.0, 1.0);
			if (uGradientReverse == 1) t = 1.0 - t;
			color = mix(uBaseColor, uTopColor, t);
		}
		gl_FragColor = vec4(color, 1.0);
	}
`

var vertShaderCode = `
	attribute vec3 position;
	attribute float aTrailT;
	uniform mat4 Pmatrix;
	uniform mat4 Vmatrix;
	uniform mat4 Mmatrix;
	uniform float uPointSize;
	varying vec3 vPosition;
	varying float vTrailT;
	void main(void) {
		gl_Position = Pmatrix * Vmatrix * Mmatrix * vec4(position, 1.0);
		gl_PointSize = uPointSize;
		vPosition = position;
		vTrailT = aTrailT;
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

	positionLoc = gl.Call("getAttribLocation", shaderProgram, "position")
	aTrailTLoc = gl.Call("getAttribLocation", shaderProgram, "aTrailT")
	gl.Call("useProgram", shaderProgram)

	// Set stride-4 attribute pointers (16 bytes per vertex: x,y,z,t)
	gl.Call("vertexAttribPointer", positionLoc, 3, glTypes.Float, false, 16, 0)
	gl.Call("enableVertexAttribArray", positionLoc)
	gl.Call("vertexAttribPointer", aTrailTLoc, 1, glTypes.Float, false, 16, 12)
	gl.Call("enableVertexAttribArray", aTrailTLoc)

	uBaseColorLoc = gl.Call("getUniformLocation", shaderProgram, "uBaseColor")
	uTopColorLoc = gl.Call("getUniformLocation", shaderProgram, "uTopColor")
	uMidColorLoc = gl.Call("getUniformLocation", shaderProgram, "uMidColor")
	uMinZLoc = gl.Call("getUniformLocation", shaderProgram, "uMinZ")
	uMaxZLoc = gl.Call("getUniformLocation", shaderProgram, "uMaxZ")
	uMinXLoc = gl.Call("getUniformLocation", shaderProgram, "uMinX")
	uMaxXLoc = gl.Call("getUniformLocation", shaderProgram, "uMaxX")
	uMinYLoc = gl.Call("getUniformLocation", shaderProgram, "uMinY")
	uMaxYLoc = gl.Call("getUniformLocation", shaderProgram, "uMaxY")
	uGradientModeLoc = gl.Call("getUniformLocation", shaderProgram, "uGradientMode")
	uGradientReverseLoc = gl.Call("getUniformLocation", shaderProgram, "uGradientReverse")
	uPointSizeLoc = gl.Call("getUniformLocation", shaderProgram, "uPointSize")
	gl.Call("uniform1f", uPointSizeLoc, 2.0)
	gl.Call("uniform3f", uBaseColorLoc, baseColor[0], baseColor[1], baseColor[2])
	gl.Call("uniform3f", uTopColorLoc, topColor[0], topColor[1], topColor[2])
	gl.Call("uniform3f", uMidColorLoc, midColor[0], midColor[1], midColor[2])
	gl.Call("uniform1f", uMinZLoc, float64(-1))
	gl.Call("uniform1f", uMaxZLoc, float64(1))
	gl.Call("uniform1f", uMinXLoc, float64(-1))
	gl.Call("uniform1f", uMaxXLoc, float64(1))
	gl.Call("uniform1f", uMinYLoc, float64(-1))
	gl.Call("uniform1f", uMaxYLoc, float64(1))
	gl.Call("uniform1i", uGradientModeLoc, 0)
	gl.Call("uniform1i", uGradientReverseLoc, 0)
	shadersReady = true

	gl.Call("clearColor", 0, 0, 0, 0)
	gl.Call("clearDepth", 1.0)
	gl.Call("viewport", 0, 0, width, height)
	gl.Call("depthFunc", glTypes.LEqual)
}

func setupMatrices() {
	// projMatrix is a pkg var (textured_js.go) so texProgram can reuse it.
	projMatrix = mgl32.Perspective(mgl32.DegToRad(45.0), float32(width)/float32(height), 1, 100.0)
	gl.Call("useProgram", shaderProgram)
	gl.Call("uniformMatrix4fv", gl.Call("getUniformLocation", shaderProgram, "Pmatrix"), false, mat4ToTyped(&projMatrix))

	movMatrix = mgl32.Ident4()
	updateViewMatrix()
	updateModelMatrix()
}

// updateViewMatrix recomputes the camera and uploads it to the attractor
// program. texProgram receives it separately (useTexProgram reads the
// pkg-level viewMatrix), so we force the attractor program active here to
// keep the upload correct even if a textured draw left texProgram bound.
func updateViewMatrix() {
	cameraPosition := mgl32.Vec3{0.0, 0.0, defaultCameraDist}
	center := mgl32.Vec3{0.0, 0.0, 0.0}
	viewMatrix = mgl32.LookAtV(cameraPosition, center, mgl32.Vec3{0.0, 1.0, 0.0})
	gl.Call("useProgram", shaderProgram)
	gl.Call("uniformMatrix4fv", gl.Call("getUniformLocation", shaderProgram, "Vmatrix"), false, mat4ToTyped(&viewMatrix))
}

func updateModelMatrix() {
	gl.Call("useProgram", shaderProgram)
	m := audioModelMatrix() // movMatrix, plus audio pump scale when active
	gl.Call("uniformMatrix4fv", gl.Call("getUniformLocation", shaderProgram, "Mmatrix"), false, mat4ToTyped(&m))
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
	// Spectrogram is a textured plane drawn through the shared 3D pipeline
	// (texProgram); update its texture and draw it, then bail out of the
	// attractor path.
	if mode == "spectrogram" {
		renderSpectrogramMode(frameNowMs)
		return
	}
	// Spectrogram skin: paint the live texture onto a surface model
	// instead of its wireframe, drawn through the same textured pipeline.
	if spectroSkin && isSkinnable(mode) {
		renderSkinnedMode(mode, frameNowMs)
		return
	}
	// xy scope draws on its own 2D program via renderAudioFrame; skip the
	// attractor pipeline entirely. This path is still reached via
	// onModeChange / buildParamPanel / paused-frame redraw, so a plain
	// return is the correct response.
	if isAudioMode(mode) {
		return
	}
	// Transitioning back into an attractor mode from audio: restore
	// the attractor useProgram binding NOW rather than waiting for the
	// next render frame, because our caller (onModeChange) is about to
	// issue drawArrays / drawElements and would draw with the wrong
	// shader program bound.
	if audioModeActive {
		deactivateAudioMode()
	}
	// Ensure the attractor program is bound — a prior spectrogram frame
	// leaves texProgram active, and the uniform/draw calls below apply to
	// whatever program is current.
	if !shaderProgram.IsUndefined() {
		gl.Call("useProgram", shaderProgram)
	}
	if shadersReady {
		gl.Call("uniform1i", uGradientModeLoc, gradientMode)
		if gradientReverse {
			gl.Call("uniform1i", uGradientReverseLoc, 1)
		} else {
			gl.Call("uniform1i", uGradientReverseLoc, 0)
		}
	}
	// Audio-reactive: modulate ODE params (dt, primary chaos param) for
	// this integration step, and the colors / point size for this frame.
	// No-op unless audio-reactive is on and the mode is an attractor.
	saved := applyAudioModulation(mode)
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
	case "tetrahedron":
		generateTetrahedron()
	case "cube":
		generateCube()
	case "octahedron":
		generateOctahedron()
	case "dodecahedron":
		generateDodecahedron()
	case "icosahedron":
		generateIcosahedron()
	case "nestedcube":
		generateNestedCube()
	case "globe":
		generateGlobe()
	case "sphere":
		generateSphere()
	case "torus":
		generateTorus()
	case "magnetosphere":
		generateMagnetosphere()
	default:
		generateRossler()
	}
	restoreAudioModulation(saved)
}

func renderLoop(this js.Value, args []js.Value) interface{} {
	// Stop button: clear once, do not reschedule. Loop dies here.
	if stopped {
		gl.Call("clearColor", 0, 0, 0, 0)
		gl.Call("clear", glTypes.ColorBufferBit)
		gl.Call("clear", glTypes.DepthBufferBit)
		return nil
	}

	// Audio modes (spectrogram, xy) render with their own shader
	// program on the shared #gocanvas. Route them here so we skip the
	// entire 3D attractor pipeline for the frame. Transition helpers
	// swap the useProgram binding when moving between attractor and
	// audio modes so neither pipeline sees the other's state.
	if len(args) > 0 {
		frameNowMs = args[0].Float() // rAF timestamp (ms), used by spectrogram scroll
	}

	// Refresh audio features (no-op unless audio-reactive is on); Phase 2
	// mappings read these to modulate the attractors.
	updateAudioFeatures()

	if isAudioMode(selectedMode) {
		if !audioModeActive {
			activateAudioMode()
		}
		renderAudioFrame(selectedMode)
		js.Global().Call("requestAnimationFrame", renderFrame)
		return nil
	}
	if audioModeActive {
		deactivateAudioMode()
	}

	now := float32(args[0].Float())
	tdiff := now - tmark
	tmark = now
	totalelapsed += tdiff

	// Debug frame timing
	if debugEnabled && lastFrameStart > 0 {
		frameMs := now - lastFrameStart
		frameCount++
		frameTotalMs += frameMs
		if frameMs < frameMinMs {
			frameMinMs = frameMs
		}
		if frameMs > frameMaxMs {
			frameMaxMs = frameMs
		}
	}
	lastFrameStart = now

	gl.Call("enable", glTypes.DepthTest)
	if !persistTrail {
		gl.Call("clear", glTypes.ColorBufferBit)
		gl.Call("clear", glTypes.DepthBufferBit)
	} else {
		// Only clear depth so new draws appear on top, but keep color buffer (old trails)
		gl.Call("clear", glTypes.DepthBufferBit)
	}

	if paused {
		// Redraw current geometry without advancing trail / auto-rotate.
		// Attractors use drawArrays with the line-strip buffer
		// (pausedCount = last frame's step count). Polyhedra +
		// geometry primitives use drawElements via generateForMode,
		// so for those we need to re-emit the geometry — drawArrays
		// alone would not consult the index buffer and the canvas
		// goes blank. Both paths skip the integrator step so the
		// visual snapshot is preserved.
		if isAttractorMode(selectedMode) {
			gl.Call("drawArrays", attractorDrawMode, 0, pausedCount)
		} else {
			generateForMode(selectedMode)
		}
		// Still allow camera interaction while paused (zoom read
		// from the Go-side cache instead of parseFloat per frame).
		zoomVal := cachedZoom
		newDist := initCameraDist - zoomVal
		if newDist != defaultCameraDist {
			defaultCameraDist = newDist
			updateViewMatrix()
		}
		if dragRotX != 0 || dragRotY != 0 {
			movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DX(dragRotX))
			movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DY(dragRotY))
			dragRotX = 0
			dragRotY = 0
			updateModelMatrix()
		}
		js.Global().Call("requestAnimationFrame", renderFrame)
		return nil
	}

	generateForMode(selectedMode)

	// Slider values come from cachedZoom/RotX/Y/Z, kept in sync by
	// input listeners in Run(). Eliminates 4 parseFloat round-trips
	// + 4 textContent writes per frame.
	zoomVal := cachedZoom
	rotationX = cachedRotX
	rotationY = cachedRotY
	rotationZ = cachedRotZ

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
	// Audio beat → rotational kick
	if spin := audioBeatSpin(); spin != 0 {
		movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DY(spin))
	}

	updateModelMatrix()
	pausedCount = steps

	js.Global().Call("requestAnimationFrame", renderFrame)
	return nil
}
