package main

import (
	"runtime"
	"syscall/js"
	"unsafe"
	"math"
	"time"
	"github.com/go-gl/mathgl/mgl32"
	"reflect"
	"strconv"
)
var (
	selected string
	selectedMode string
	renderAttractorFrame js.Func
	tmark float32
	defaultcameraDist, cameraDist float32 = float32(100), float32(0)
	x, y, z float32 = float32(0.1), float32(0.5), float32(-0.6)
	rosslerDT, rosslerA, rosslerB, rosslerC float32 = float32(0.005), float32(0.2), float32(0.2), float32(5.7)
	chuaDT, chuaA, chuaB, chuaC float32 = float32(0.005), float32(40), float32(3.0), float32(28.0)
	luchenDT, luxhenA, luchenB, luchenC float32 = float32(0.005), float32(40), float32(3.0), float32(28.0)
	aizawaDT, aizawaA, aizawaB, aizawaC, aizawaD, aizawaE, aizawaF float32 = float32(0.0052), float32(0.95), float32(0.7), float32(0.6), float32(3.5), float32(0.25), float32(0.1)
	sprottDT, sprottA, sprottB float32 = float32(0.01), float32(2.07), float32(1.8)
	rotationX, rotationY, rotationZ float32 = float32(0.0), float32(0.0), float32(0.0)
	rotationX1, rotationY1, rotationZ1 float32 = float32(0.0), float32(0.0), float32(0.0)
	lorenzDT, lorenzS, lorenzR, lorenzB float32 = float32(0.005), float32(10.0), float32(28.0), float32(2.7)
	attractorVertices     []float32
	attractorIndices []uint16
	movMatrix mgl32.Mat4
	attractorVertexBuffer js.Value = gl.Call("createBuffer")
	attractorIndexBuffer js.Value = gl.Call("createBuffer")
	doc js.Value = js.Global().Get("document")
	body js.Value = doc.Get("body")
	rtc js.Value = doc.Call("getElementById", "runtime")
	cameraControl  js.Value = doc.Call("getElementById", "camera-zoom")
	rotationControlsX  js.Value = doc.Call("getElementById", "rotation-controls-x")
	rotationControlsY  js.Value = doc.Call("getElementById", "rotation-controls-y")
	rotationControlsZ  js.Value = doc.Call("getElementById", "rotation-controls-z")
	sliderZoom js.Value = doc.Call("getElementById", "slider-value-zoom")
	sliderX js.Value = doc.Call("getElementById", "slider-value-x")
	sliderY js.Value = doc.Call("getElementById", "slider-value-y")
	sliderZ js.Value = doc.Call("getElementById", "slider-value-z")
	//	statusDiv js.Value = doc.Call("getElementById", "status")
//	lenattractorvDiv js.Value = doc.Call("getElementById", "lenattractorv")
//	stepsDiv js.Value = doc.Call("getElementById", "steps")
	canvasEl js.Value = doc.Call("getElementById", "gocanvas")
	width int = doc.Get("body").Get("clientWidth").Int()
	height int = doc.Get("body").Get("clientHeight").Int()
	gl js.Value = canvasEl.Call("getContext", "webgl")
	steps int = 100000
	shaderProgram js.Value = gl.Call("createProgram")
	totalelapsed float32
)
var fragShaderCode string = `
	precision mediump float;
	uniform vec3 uBaseColor; // Color value at the base
	uniform vec3 uTopColor;  // Color value at the top
	varying vec3 vPosition;  // Interpolated vertex position
	void main(void) {
		float t = (vPosition.z + 1.0) * 0.5; // Normalize the y-coordinate to [0, 1]
		vec3 rainbowColor = mix(uBaseColor, uTopColor, t);
		gl_FragColor = vec4(rainbowColor, 1.0);
	}
`
var fragShaderCode1 string = `
	precision mediump float;
	uniform vec3 uGreenColor;  // Color value at the top
	varying vec3 vPosition;  // Interpolated vertex position
	void main(void) {
		gl_FragColor = vec4(uGreenColor, 1.0);
	}
`
var vertShaderCode string = `
	attribute vec3 position;
	uniform mat4 Pmatrix;
	uniform mat4 Vmatrix;
	uniform mat4 Mmatrix;
	varying vec3 vPosition;  // Pass vertex position to fragment shader
	void main(void) {
		gl_Position = Pmatrix * Vmatrix * Mmatrix * vec4(position, 1.0);
		vPosition = position;  // Pass vertex position to fragment shader
	}
`
func init () {
	gl = canvasEl.Call("getContext", "webgl")
	canvasEl.Set("width", width)
	canvasEl.Set("height", height)
	if gl.IsUndefined() {	gl = canvasEl.Call("getContext", "experimental-webgl")	}
	if gl.IsUndefined() {
		js.Global().Call("alert", "browser might not support webgl")
		return
	}
}
func updateOutput(this js.Value, p []js.Value) interface{} {
	selectedMode = doc.Call("querySelector", "input[name=radio-group]:checked").Get("value").String()
	outputDiv := doc.Call("getElementById", "output")
	outputDiv.Set("textContent", selectedMode)
//	wg.Done() // Signal that the user's selection is complete
	return nil
}
func main() {
	if body.IsUndefined() { body = doc.Get("body") }
	if body.IsUndefined() {
		js.Global().Call("alert", "cannot get html body, exiting")
		return
	}
	rawHTML1 := `
	<body style="margin: 0; padding: 0; width: 100%; height: 100%; background-color: black; color: white;">
		<div id='gocanvas-container' style="position: absolute; width: 100%; height: 100%; pointer-events: none; z-index: 3;">
			<canvas id='gocanvas' style="max-width: 100%; max-height: 100%; z-index: 3;"></canvas>
		</div>
	<div id="radio-container">
	<label for="radio-rossler">Rossler</label>
	<input type="radio" name="radio-group" id="radio-rossler" value="rossler">
	<label for="radio-lorenz">Lorenz</label>
	<input type="radio" name="radio-group" id="radio-lorenz" value="lorenz">
	<label for="radio-chua">Chua</label>
	<input type="radio" name="radio-group" id="radio-chua" value="chua">
	<label for="radio-aizawa">Aizawa</label>
	<input type="radio" name="radio-group" id="radio-aizawa" value="aizawa">
	<label for="radio-sprott">Sprott</label>
	<input type="radio" name="radio-group" id="radio-sprott" value="sprott">
	<label for="radio-lissajou">Lissajou</label>
	<input type="radio" name="radio-group" id="radio-lissajou" value="lissajou">
	<label for="radio-cube">Cube</label>
	<input type="radio" name="radio-group" id="radio-cube" value="cube">
	<label for="radio-sphere">Sphere</label>
	<input type="radio" name="radio-group" id="radio-sphere" value="sphere">
	</div>
	<div id="output"></div>
	</body>
	`
	div := doc.Call("createElement", "div")
	div.Set("innerHTML", rawHTML1)
	body.Call("appendChild", div)
	doc = js.Global().Get("document")
	body = doc.Get("body")
	selectedMode = ""
//	var wg sync.WaitGroup
//	wg.Add(1)

	updateOutputCallback := js.FuncOf(updateOutput)

	defer updateOutputCallback.Release()



	radioLorenz := doc.Call("getElementById", "radio-lorenz")
	radioLorenz.Call("addEventListener", "change", updateOutputCallback)
	radioRossler := doc.Call("getElementById", "radio-rossler")
	radioRossler.Call("addEventListener", "change", updateOutputCallback)
	radioChua := doc.Call("getElementById", "radio-chua")
	radioChua.Call("addEventListener", "change", updateOutputCallback)
	radioAizawa := doc.Call("getElementById", "radio-aizawa")
	radioAizawa.Call("addEventListener", "change", updateOutputCallback)
	radioSprott := doc.Call("getElementById", "radio-sprott")
	radioSprott.Call("addEventListener", "change", updateOutputCallback)
	radioLissajou := doc.Call("getElementById", "radio-lissajou")
	radioLissajou.Call("addEventListener", "change", updateOutputCallback)
	radioCube := doc.Call("getElementById", "radio-cube")
	radioCube.Call("addEventListener", "change", updateOutputCallback)
	radioSphere := doc.Call("getElementById", "radio-sphere")
	radioSphere.Call("addEventListener", "change", updateOutputCallback)


//	wg.Wait() // Wait for the user's selection

	naim()

}
func naim() {
	doc = js.Global().Get("document")
	body = doc.Get("body")
updateState := js.FuncOf(func(this js.Value, p []js.Value) interface{} {
selected = p[0].Get("target").Get("value").String()
stateDiv := doc.Call("getElementById", "state")
stateDiv.Set("textContent", selected)
return nil
})
defer updateState.Release()

rawHTML := `

<div class="controls">
<label for="camera-zoom">Zoom.</label>
<input type="range" id="camera-zoom" class="rotation-slider" min="-1" max="1" value="0" step="0.1">
<output id="slider-value-zoom">0</output>
</div>
<div class="controls">
<label for="rotation-controls-x">X-Axis</label>
<input type="range" id="rotation-controls-x" class="rotation-slider" min="-1" max="1" value="0" step="0.1">
<output id="slider-value-x">0</output>
</div>
<div class="controls">
<label for="rotation-controls-y">Y-Axis</label>
<input type="range" id="rotation-controls-y" class="rotation-slider" min="-1" max="1" value="0" step="0.1">
<output id="slider-value-y">0</output>
</div>
<div class="controls">
<label for="rotation-controls-z">Z-Axis</label>
<input type="range" id="rotation-controls-z" class="rotation-slider" min="-1" max="1" value="0" step="0.1">
<output id="slider-value-z">0</output>
</div>
<div id="runtime">Clock</div>
`

	div := doc.Call("createElement", "div")
	div.Set("innerHTML", rawHTML)
	doc = js.Global().Get("document")

	body.Call("appendChild", div)
	doc = js.Global().Get("document")
	body = doc.Get("body")

	rtc = 	doc.Call("getElementById", "runtime")
	cameraControl = doc.Call("getElementById", "camera-zoom")
	rotationControlsX = doc.Call("getElementById", "rotation-controls-x")
	rotationControlsY = doc.Call("getElementById", "rotation-controls-y")
	rotationControlsZ = doc.Call("getElementById", "rotation-controls-z")
	cameraDist = float32(js.Global().Get("parseFloat").Invoke(cameraControl.Get("value")).Float())
	rotationX = float32(js.Global().Get("parseFloat").Invoke(rotationControlsX.Get("value")).Float())
	rotationY = float32(js.Global().Get("parseFloat").Invoke(rotationControlsY.Get("value")).Float())
	rotationZ = float32(js.Global().Get("parseFloat").Invoke(rotationControlsZ.Get("value")).Float())
	sliderZoom = doc.Call("getElementById", "slider-value-zoom")
	sliderX = doc.Call("getElementById", "slider-value-x")
	sliderY = doc.Call("getElementById", "slider-value-y")
	sliderZ = doc.Call("getElementById", "slider-value-z")
	if !sliderZoom.IsUndefined() {	    sliderZoom.Set("textContent", strconv.FormatFloat(float64(cameraDist), 'f', 1, 64))	}
	if !sliderX.IsUndefined() {	    sliderX.Set("textContent", strconv.FormatFloat(float64(rotationX), 'f', 1, 64))	}
	if !sliderY.IsUndefined() {	    sliderY.Set("textContent", strconv.FormatFloat(float64(rotationY), 'f', 1, 64))	}
	if !sliderZ.IsUndefined() {	    sliderZ.Set("textContent", strconv.FormatFloat(float64(rotationZ), 'f', 1, 64))	}
	if !rtc.IsUndefined() {		rtc.Set("innerHTML", time.Now().Format("2006-02-01 15:04:05")) }
	glTypes.New(gl)

		switch sel := selectedMode; sel {
		case "lorenz":
			generateLorenz(lorenzDT, lorenzS, lorenzR, lorenzB)
		case "rossler":
			generateRossler(rosslerDT, rosslerA, rosslerB, rosslerC)
		case "chua":
			generateChua(chuaDT, chuaA, chuaB, chuaC)
		case "aizawa":
			generateAizawa(aizawaDT, aizawaA, aizawaB, aizawaC, aizawaD, aizawaE, aizawaF)
		case "lissajou":
			generateLissajou(float32(9), float32(4), float32(25))
		case "sprott":
			generateSprott(sprottDT, sprottA, sprottB)
		case "cube":
			generateCube()
		case "sphere":
			radius := float32(1.0)
			stacks := 30
			slices := 30
			generateSphere(radius, stacks, slices)
			generateTorus(radius, radius, stacks, slices)
			generateTorus(radius*2, radius, stacks, slices)
		default:
			generateRossler(rosslerDT, rosslerA, rosslerB, rosslerC)
		}

	vertShader := gl.Call("createShader", glTypes.VertexShader)
	gl.Call("shaderSource", vertShader, vertShaderCode)
	gl.Call("compileShader", vertShader)
	fragShader := gl.Call("createShader", glTypes.FragmentShader)
	fragShader1 := gl.Call("createShader", glTypes.FragmentShader)
	gl.Call("shaderSource", fragShader, fragShaderCode)
	gl.Call("compileShader", fragShader)
	gl.Call("shaderSource", fragShader1, fragShaderCode1)
	gl.Call("compileShader", fragShader1)
	gl.Call("attachShader", shaderProgram, vertShader)
	gl.Call("attachShader", shaderProgram, fragShader)
	gl.Call("attachShader", shaderProgram, fragShader1)
	gl.Call("linkProgram", shaderProgram)
	position := gl.Call("getAttribLocation", shaderProgram, "position")
	gl.Call("vertexAttribPointer", position, 3, glTypes.Float, false, 0, 0)
	gl.Call("enableVertexAttribArray", position)
	gl.Call("useProgram", shaderProgram)
	uBaseColor := gl.Call("getUniformLocation", shaderProgram, "uBaseColor")
	uTopColor := gl.Call("getUniformLocation", shaderProgram, "uTopColor")
	uGreenColor := gl.Call("getUniformLocation", shaderProgram, "uGreenColor")
	uColor := gl.Call("getUniformLocation", shaderProgram, "uColor")
	gl.Call("uniform3f", uBaseColor, 1.0, 0.0, 0.0)
	gl.Call("uniform3f", uTopColor, 0.0, 0.0, 1.0)
	gl.Call("uniform3f", uGreenColor, 0.0, 1.0, 0.0)
	gl.Call("uniform3f", uColor, 1.0, 1.0, 1.0)
	gl.Call("clearColor", 0, 0, 0, 0)
	gl.Call("clearDepth", 1.0)
	gl.Call("viewport", 0, 0, width, height)
	gl.Call("depthFunc", glTypes.LEqual)
	projMatrix := mgl32.Perspective(mgl32.DegToRad(45.0), float32(width)/float32(height), 1, 100.0)
	projMatrixBuffer := (*[16]float32)(unsafe.Pointer(&projMatrix))
	typedProjMatrixBuffer := SliceToTypedArray([]float32((*projMatrixBuffer)[:]))
	gl.Call("uniformMatrix4fv", gl.Call("getUniformLocation", shaderProgram, "Pmatrix"), false, typedProjMatrixBuffer)

	movMatrix = mgl32.Ident4()
	cameraPosition := mgl32.Vec3{0.0, 0.0, defaultcameraDist} // Set the initial camera position
	center := mgl32.Vec3{0.0, 0.0, 0.0}
	for i := 0; i < len(attractorVertices); i += 3 {
		center[0] += attractorVertices[i]
		center[1] += attractorVertices[i+1]
		center[2] += attractorVertices[i+2]
	}
	center = center.Mul(1.0 / float32(len(attractorVertices)/3))
	cameraDirection := center.Sub(cameraPosition).Normalize()
	upVector := mgl32.Vec3{0.0, 1.0, 0.0}
	rightVector := cameraDirection.Cross(upVector).Normalize()
	newUpVector := rightVector.Cross(cameraDirection)
	viewMatrix := mgl32.LookAtV(cameraPosition, center, newUpVector)
	viewMatrixBuffer := (*[16]float32)(unsafe.Pointer(&viewMatrix))
	typedViewMatrixBuffer := SliceToTypedArray([]float32((*viewMatrixBuffer)[:]))
	gl.Call("uniformMatrix4fv", gl.Call("getUniformLocation", shaderProgram, "Vmatrix"), false, typedViewMatrixBuffer)
	modelMatrixBuffer := (*[16]float32)(unsafe.Pointer(&movMatrix))
	typedModelMatrixBuffer := SliceToTypedArray([]float32((*modelMatrixBuffer)[:]))
	gl.Call("uniformMatrix4fv", gl.Call("getUniformLocation", shaderProgram, "Mmatrix"), false, typedModelMatrixBuffer)

	done := make(chan struct{}, 0)
	renderAttractorFrame = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
    now := float32(args[0].Float())
    tdiff := now - tmark
    tmark = now
		totalelapsed += tdiff
		gl.Call("enable", glTypes.DepthTest)
		gl.Call("clear", glTypes.ColorBufferBit)
		gl.Call("clear", glTypes.DepthBufferBit)

				switch sel := selectedMode; sel {
				case "lorenz":
					generateLorenz(lorenzDT, lorenzS, lorenzR, lorenzB)
				case "rossler":
					generateRossler(rosslerDT, rosslerA, rosslerB, rosslerC)
				case "chua":
					generateChua(chuaDT, chuaA, chuaB, chuaC)
				case "aizawa":
					generateAizawa(aizawaDT, aizawaA, aizawaB, aizawaC, aizawaD, aizawaE, aizawaF)
				case "lissajou":
					generateLissajou(float32(9), float32(4), float32(25))
				case "sprott":
					generateSprott(sprottDT, sprottA, sprottB)
				case "cube":
					generateCube()
				case "sphere":
					radius := float32(1.0)
					stacks := 30
					slices := 30
					generateSphere(radius, stacks, slices)
					generateTorus(radius, radius, stacks, slices)
					generateTorus(radius*2, radius*2, stacks, slices)
				default:
					generateRossler(rosslerDT, rosslerA, rosslerB, rosslerC)
				}

		cameraDist = float32(js.Global().Get("parseFloat").Invoke(cameraControl.Get("value")).Float())
		rotationX = float32(js.Global().Get("parseFloat").Invoke(rotationControlsX.Get("value")).Float())
    rotationY = float32(js.Global().Get("parseFloat").Invoke(rotationControlsY.Get("value")).Float())
    rotationZ = float32(js.Global().Get("parseFloat").Invoke(rotationControlsZ.Get("value")).Float())
	    sliderZoom.Set("textContent", strconv.FormatFloat(float64(cameraDist), 'f', 1, 64))
	    sliderX.Set("textContent", strconv.FormatFloat(float64(rotationX), 'f', 1, 64))
	    sliderY.Set("textContent", strconv.FormatFloat(float64(rotationY), 'f', 1, 64))
	    sliderZ.Set("textContent", strconv.FormatFloat(float64(rotationZ), 'f', 1, 64))
		if cameraDist != 0 {
			defaultcameraDist += cameraDist
			cameraPosition := mgl32.Vec3{0.0, 0.0, defaultcameraDist}
			center = center.Mul(1.0 / float32(len(attractorVertices)/3))
			cameraDirection := center.Sub(cameraPosition).Normalize()
			upVector := mgl32.Vec3{0.0, 1.0, 0.0}
			rightVector := cameraDirection.Cross(upVector).Normalize()
			newUpVector := rightVector.Cross(cameraDirection)
			viewMatrix := mgl32.LookAtV(cameraPosition, center, newUpVector)
			viewMatrixBuffer := (*[16]float32)(unsafe.Pointer(&viewMatrix))
			typedViewMatrixBuffer := SliceToTypedArray([]float32((*viewMatrixBuffer)[:]))
			gl.Call("uniformMatrix4fv", gl.Call("getUniformLocation", shaderProgram, "Vmatrix"), false, typedViewMatrixBuffer)
		}
		if rotationX != 0 {
			rotationX1 = rotationX/20 + float32(tdiff) * 0.00000001
			movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DX(rotationX1))
		}
		if rotationY != 0 {
			rotationY1 = rotationY/20 + float32(tdiff) * 0.00000001
			movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DY(rotationY1))
		}
		if rotationZ != 0 {
			rotationZ1 = rotationZ/20 + float32(tdiff) * 0.00000001
			movMatrix = movMatrix.Mul4(mgl32.HomogRotate3DZ(rotationZ1))
		}
		modelMatrixBuffer := (*[16]float32)(unsafe.Pointer(&movMatrix))
		typedModelMatrixBuffer := SliceToTypedArray([]float32((*modelMatrixBuffer)[:]))
		gl.Call("uniformMatrix4fv", gl.Call("getUniformLocation", shaderProgram, "Mmatrix"), false, typedModelMatrixBuffer)
    js.Global().Call("requestAnimationFrame", renderAttractorFrame)
//		if !rtc.IsUndefined() {		rtc.Set("innerHTML", time.Now().Format("2006-02-01 15:04:05")) }
    return nil
})
	defer renderAttractorFrame.Release()
	js.Global().Call("requestAnimationFrame", renderAttractorFrame)
	doneTime := make(chan struct{})
go func() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-doneTime:
			return
		case <-ticker.C:
			if !rtc.IsUndefined() {		rtc.Set("innerHTML", time.Now().Format("2006-02-01 15:04:05")) }
		}
	}
}()
	<-done
		close(doneTime)
main()
}


var verticesCube = []float32{
	-1, -1, -1, 1, -1, -1, 1, 1, -1, -1, 1, -1,
	-1, -1, 1, 1, -1, 1, 1, 1, 1, -1, 1, 1,
	-1, -1, -1, -1, 1, -1, -1, 1, 1, -1, -1, 1,
	1, -1, -1, 1, 1, -1, 1, 1, 1, 1, -1, 1,
	-1, -1, -1, -1, -1, 1, 1, -1, 1, 1, -1, -1,
	-1, 1, -1, -1, 1, 1, 1, 1, 1, 1, 1, -1,
	-0.5, -0.5, -0.5, 0.5, -0.5, -0.5, 0.5, 0.5, -0.5, -0.5, 0.5, -0.5,
	-0.5, -0.5, 0.5, 0.5, -0.5, 0.5, 0.5, 0.5, 0.5, -0.5, 0.5, 0.5,
	-0.5, -0.5, -0.5, -0.5, 0.5, -0.5, -0.5, 0.5, 0.5, -0.5, -0.5, 0.5,
	0.5, -0.5, -0.5, 0.5, 0.5, -0.5, 0.5, 0.5, 0.5, 0.5, -0.5, 0.5,
	-0.5, -0.5, -0.5, -0.5, -0.5, 0.5, 0.5, -0.5, 0.5, 0.5, -0.5, -0.5,
	-0.5, 0.5, -0.5, -0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, -0.5,

}
var colorsNative = []float32{
	5, 3, 7, 5, 3, 7, 5, 3, 7, 5, 3, 7,
	1, 1, 3, 1, 1, 3, 1, 1, 3, 1, 1, 3,
	0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1,
	1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0,
	1, 1, 0, 1, 1, 0, 1, 1, 0, 1, 1, 0,
	0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0,
}
var indicesCube = []uint16{
	0, 1, 2, 1, 2, 3, 4, 5, 6, 5, 6, 7,
	8, 9, 10, 9, 10, 11, 12, 13, 14, 13, 14, 15,
	16, 17, 18, 17, 18, 19, 20, 21, 22, 21, 22, 23,
	24, 25, 26, 25, 26, 27, 28, 29, 30, 29, 30, 31,
	32, 33, 34, 33, 34, 35, 36, 37, 38, 37, 38, 39,
	40, 41, 42, 41, 42, 43, 44, 45, 46, 45, 46, 47,
}


func generateCube() {
	attractorVertices = verticesCube
	attractorIndices = indicesCube
	gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
	gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(attractorVertices), glTypes.StaticDraw)
	gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
	gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(attractorIndices), glTypes.StaticDraw)
	gl.Call("drawElements", glTypes.Line, len(attractorIndices), glTypes.UnsignedShort, 0)
}
func generateSphere(radius float32, stacks, slices int) {
	var vertices []float32
	var indices []uint16
	for i := 0; i <= stacks; i++ {
		phi := float32(i) * float32(math.Pi) / float32(stacks)
		for j := 0; j <= slices; j++ {
			theta := float32(j) * 2.0 * float32(math.Pi) / float32(slices)
			x := radius * float32(math.Sin(float64(phi))) * float32(math.Cos(float64(theta)))
			y := radius * float32(math.Sin(float64(phi))) * float32(math.Sin(float64(theta)))
			z := radius * float32(math.Cos(float64(phi)))
			vertices = append(vertices, x, y, z)
		}
	}
	for i := 0; i < stacks; i++ {
		for j := 0; j <= slices; j++ {
			indices = append(indices, uint16(i*(slices+1)+j), uint16((i+1)*(slices+1)+j))
		}
	}
	attractorVertices = vertices
	attractorIndices = indices
	gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
	gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(attractorVertices), glTypes.StaticDraw)
	gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
	gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(attractorIndices), glTypes.StaticDraw)
	gl.Call("drawElements", glTypes.Line, len(attractorIndices), glTypes.UnsignedShort, 0)
//gl.Call("drawElements", glTypes.Line, len(indices), glTypes.UnsignedShort, 0)
	// Change the drawing method to draw lines instead of triangles
	gl.Call("drawArrays", glTypes.LineLoop, 0, len(attractorVertices)/3)

}
func generateTorus(radius, minorRadius float32, stacks, slices int) {
	var vertices []float32
	var indices []uint16
	for i := 0; i < stacks; i++ {
	    theta := float32(i) * 2.0 * math.Pi / float32(stacks)
	    for j := 0; j <= slices; j++ {
	        phi := float32(j) * 2.0 * math.Pi / float32(slices)
	        //x := (radius + minorRadius*float32(math.Cos(float64(phi)))) * float32(math.Cos(float64(theta)))
	        //y := (radius + minorRadius*float32(math.Cos(float64(phi)))) * float32(math.Sin(float64(theta)))
	        //z := minorRadius * float32(math.Sin(float64(phi)))
	        vertices = append(vertices, float32((radius + minorRadius*float32(math.Cos(float64(phi)))) * float32(math.Cos(float64(theta)))), float32((radius + minorRadius*float32(math.Cos(float64(phi)))) * float32(math.Sin(float64(theta)))), float32(minorRadius * float32(math.Sin(float64(phi)))))
	    }
	}
	for j := 0; j < stacks; j++ {
	    for i := 0; i < slices; i++ {
	        first := uint16((j * (slices + 1)) + i)
	        second := first + 1
	        third := first + uint16(slices) + 1
	        fourth := third + 1

	        indices = append(indices, first, second, third)
	        indices = append(indices, second, third, fourth)
	    }
}
	attractorVertices = vertices
	attractorIndices = indices
	gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
	gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(attractorVertices), glTypes.StaticDraw)
	gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
	gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(attractorIndices), glTypes.StaticDraw)
//	gl.Call("drawElements", glTypes.Line, len(attractorIndices), glTypes.UnsignedShort, 0)
//	gl.Call("drawElements", glTypes.Line, len(attractorIndices), glTypes.UnsignedShort, 0)
	// Change the drawing method to draw lines instead of triangles
//	gl.Call("drawArrays", glTypes.LineStrip, 0, int32(len(attractorVertices)/3))
gl.Call("drawArrays", glTypes.LineLoop, 0, len(attractorVertices)/3)

}

func generateLorenz(dt, s, r, b float32) {
vertices := make([]float32, steps*3)
for i := 0; i < steps; i++ {
	x1 := x + dt*s*(y-x)
	y1 := y + dt*(x*(r-z)-y)
	z1 := z + dt*(x*y-b*z)
	x, y, z = x1, y1, z1
	vertices[i*3] = x
	vertices[i*3+1] = y
	vertices[i*3+2] = z
}
attractorVertices = vertices
var indices []uint16
	for i := 0; i < steps; i++ {		indices = append(indices, uint16(i), uint16(i+1))	}
	attractorIndices = indices
gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(attractorVertices), glTypes.StaticDraw)
gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(attractorIndices), glTypes.StaticDraw)
gl.Call("drawArrays", glTypes.LineStrip, 0, int32(len(attractorVertices)/3))
}

func generateRossler(dt, a, b, c float32) {
vertices := make([]float32, steps*3)
for i := 0; i < steps; i++ {
x1 := x + dt*(-y-z)
y1 := y + dt*(x+a*y)
z1 := z + dt*(b+z*(x-c))
x, y, z = x1, y1, z1
vertices[i*3] = x
vertices[i*3+1] = y
vertices[i*3+2] = z
}
attractorVertices = vertices
var indices []uint16
for i := 0; i < steps; i++ {		indices = append(indices, uint16(i), uint16(i+1))	}
attractorIndices = indices
gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(attractorVertices), glTypes.StaticDraw)
gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(attractorIndices), glTypes.StaticDraw)
gl.Call("drawArrays", glTypes.LineStrip, 0, int32(len(attractorVertices)/3))
}
func generateChua(dt, a, b, c float32) {
vertices := make([]float32, steps*3)
x, y, z := float32(0.1), float32(0), float32(0)
for i := 0; i < steps; i++ {
	x1 := x + dt*a*(y-x-b*z)
	y1 := y + dt*(x-x*y-z)
	z1 := z + dt*(b*y-z)
	x, y, z = x1, y1, z1
	vertices[i*3] = x
	vertices[i*3+1] = y
	vertices[i*3+2] = z
}
attractorVertices = vertices
var indices []uint16
for i := 0; i < steps-1; i++ {
	indices = append(indices, uint16(i), uint16(i+1))
}
attractorIndices = indices
gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(attractorVertices), glTypes.StaticDraw)
gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(attractorIndices), glTypes.StaticDraw)
gl.Call("drawArrays", glTypes.LineStrip, 0, int32(len(attractorVertices)/3))
}
func generateSprott(dt, a, b float32) {
    vertices := make([]float32, steps*3)
    for i := 0; i < steps; i++ {
        x1 := x + dt * (y + a * x * y + x * z)
        y1 := y + dt * (1 - b * x * x + y * z)
        z1 := z + dt * (x - x * x - y * y)
        x, y, z = x1, y1, z1
        vertices[i*3] = x
        vertices[i*3+1] = y
        vertices[i*3+2] = z
    }
    attractorVertices = vertices

    var indices []uint16
    for i := 0; i < steps-1; i++ {
        indices = append(indices, uint16(i), uint16(i+1))
    }
    attractorIndices = indices

    gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
    gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(attractorVertices), glTypes.StaticDraw)
    gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
    gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(attractorIndices), glTypes.StaticDraw)
    gl.Call("drawArrays", glTypes.LineStrip, 0, int32(len(attractorVertices)/3))
}

func generateAizawa(dt, a, b, c, d, e, f float32) {
vertices := make([]float32, steps*3)
for i := 0; i < steps; i++ {
	x1 := x + dt * ((z - b) * x - d * y)
	y1 := y + dt * (d * x + (z - b) * y)
	z1 := z + dt * (c + a * z - (z*z*z) / 3 - (x*x + y*y) * (1 + e * z) + f * z * x*x*x)
	x, y, z = x1, y1, z1
	vertices[i*3] = x
	vertices[i*3+1] = y
	vertices[i*3+2] = z
}
attractorVertices = vertices
var indices []uint16
for i := 0; i < steps; i++ {
	indices = append(indices, uint16(i), uint16(i+1))
}
attractorIndices = indices
gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(attractorVertices), glTypes.StaticDraw)
gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(attractorIndices), glTypes.StaticDraw)
gl.Call("drawArrays", glTypes.LineStrip, 0, int32(len(attractorVertices)/3))
}

func generateLissajou(a, b, c float32) {
vertices := make([]float32, steps*3)
for i := 0; i < steps; i++ {
	t := float32(i) * (2 * math.Pi) / float32(steps)
	x := math.Sin(float64(a*t))
	y := math.Sin(float64(b*t))
	z := math.Sin(float64(c*t))
	vertices[i*3] = float32(x)
	vertices[i*3+1] = float32(y)
	vertices[i*3+2] = float32(z)
}
attractorVertices = vertices
var indices []uint16
for i := 0; i < steps-1; i++ {
	indices = append(indices, uint16(i), uint16(i+1))
}
attractorIndices = indices
gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(attractorVertices), glTypes.StaticDraw)
gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(attractorIndices), glTypes.StaticDraw)
gl.Call("drawArrays", glTypes.LineStrip, 0, int32(len(attractorVertices)/3))
}

var glTypes GLTypes
// GLTypes provides WebGL bindings.
type GLTypes struct {
	StaticDraw         js.Value
	ArrayBuffer        js.Value
	ElementArrayBuffer js.Value
	VertexShader       js.Value
	FragmentShader     js.Value
	Float              js.Value
	DepthTest          js.Value
	ColorBufferBit     js.Value
	DepthBufferBit     js.Value
	Triangles          js.Value
	UnsignedShort      js.Value
	LEqual             js.Value
	LineLoop           js.Value
	Line               js.Value
	LineStrip          js.Value
	DynamicDraw        js.Value
}

func (types *GLTypes) New(gl js.Value) {
	types.StaticDraw = gl.Get("STATIC_DRAW")
	types.ArrayBuffer = gl.Get("ARRAY_BUFFER")
	types.ElementArrayBuffer = gl.Get("ELEMENT_ARRAY_BUFFER")
	types.VertexShader = gl.Get("VERTEX_SHADER")
	types.FragmentShader = gl.Get("FRAGMENT_SHADER")
	types.Float = gl.Get("FLOAT")
	types.DepthTest = gl.Get("DEPTH_TEST")
	types.ColorBufferBit = gl.Get("COLOR_BUFFER_BIT")
	types.Triangles = gl.Get("TRIANGLES")
	types.UnsignedShort = gl.Get("UNSIGNED_SHORT")
	types.LEqual = gl.Get("LEQUAL")
	types.DepthBufferBit = gl.Get("DEPTH_BUFFER_BIT")
	types.LineLoop = gl.Get("LINE_LOOP")
	types.Line = gl.Get("LINES")
	types.LineStrip = gl.Get("LINE_STRIP")
	types.DynamicDraw = gl.Get("DYNAMIC_DRAW")
}

func sliceToByteSlice(s interface{}) []byte {
	switch s := s.(type) {
	case []int8:
		h := (*reflect.SliceHeader)(unsafe.Pointer(&s))
		return *(*[]byte)(unsafe.Pointer(h))
	case []int16:
		h := (*reflect.SliceHeader)(unsafe.Pointer(&s))
		h.Len *= 2
		h.Cap *= 2
		return *(*[]byte)(unsafe.Pointer(h))
	case []int32:
		h := (*reflect.SliceHeader)(unsafe.Pointer(&s))
		h.Len *= 4
		h.Cap *= 4
		return *(*[]byte)(unsafe.Pointer(h))
	case []int64:
		h := (*reflect.SliceHeader)(unsafe.Pointer(&s))
		h.Len *= 8
		h.Cap *= 8
		return *(*[]byte)(unsafe.Pointer(h))
	case []uint8:
		return s
	case []uint16:
		h := (*reflect.SliceHeader)(unsafe.Pointer(&s))
		h.Len *= 2
		h.Cap *= 2
		return *(*[]byte)(unsafe.Pointer(h))
	case []uint32:
		h := (*reflect.SliceHeader)(unsafe.Pointer(&s))
		h.Len *= 4
		h.Cap *= 4
		return *(*[]byte)(unsafe.Pointer(h))
	case []uint64:
		h := (*reflect.SliceHeader)(unsafe.Pointer(&s))
		h.Len *= 8
		h.Cap *= 8
		return *(*[]byte)(unsafe.Pointer(h))
	case []float32:
		h := (*reflect.SliceHeader)(unsafe.Pointer(&s))
		h.Len *= 4
		h.Cap *= 4
		return *(*[]byte)(unsafe.Pointer(h))
	case []float64:
		h := (*reflect.SliceHeader)(unsafe.Pointer(&s))
		h.Len *= 8
		h.Cap *= 8
		return *(*[]byte)(unsafe.Pointer(h))
	default:
		panic("jsutil: unexpected value at sliceToBytesSlice: " + js.ValueOf(s).Type().String())
	}
}

func SliceToTypedArray(s interface{}) js.Value {
	switch s := s.(type) {
	case []int8:
		a := js.Global().Get("Uint8Array").New(len(s))
		js.CopyBytesToJS(a, sliceToByteSlice(s))
		runtime.KeepAlive(s)
		buf := a.Get("buffer")
		return js.Global().Get("Int8Array").New(buf, a.Get("byteOffset"), a.Get("byteLength"))
	case []int16:
		a := js.Global().Get("Uint8Array").New(len(s) * 2)
		js.CopyBytesToJS(a, sliceToByteSlice(s))
		runtime.KeepAlive(s)
		buf := a.Get("buffer")
		return js.Global().Get("Int16Array").New(buf, a.Get("byteOffset"), a.Get("byteLength").Int()/2)
	case []int32:
		a := js.Global().Get("Uint8Array").New(len(s) * 4)
		js.CopyBytesToJS(a, sliceToByteSlice(s))
		runtime.KeepAlive(s)
		buf := a.Get("buffer")
		return js.Global().Get("Int32Array").New(buf, a.Get("byteOffset"), a.Get("byteLength").Int()/4)
	case []uint8:
		a := js.Global().Get("Uint8Array").New(len(s))
		js.CopyBytesToJS(a, s)
		runtime.KeepAlive(s)
		return a
	case []uint16:
		a := js.Global().Get("Uint8Array").New(len(s) * 2)
		js.CopyBytesToJS(a, sliceToByteSlice(s))
		runtime.KeepAlive(s)
		buf := a.Get("buffer")
		return js.Global().Get("Uint16Array").New(buf, a.Get("byteOffset"), a.Get("byteLength").Int()/2)
	case []uint32:
		a := js.Global().Get("Uint8Array").New(len(s) * 4)
		js.CopyBytesToJS(a, sliceToByteSlice(s))
		runtime.KeepAlive(s)
		buf := a.Get("buffer")
		return js.Global().Get("Uint32Array").New(buf, a.Get("byteOffset"), a.Get("byteLength").Int()/4)
	case []float32:
		a := js.Global().Get("Uint8Array").New(len(s) * 4)
		js.CopyBytesToJS(a, sliceToByteSlice(s))
		runtime.KeepAlive(s)
		buf := a.Get("buffer")
		return js.Global().Get("Float32Array").New(buf, a.Get("byteOffset"), a.Get("byteLength").Int()/4)
	case []float64:
		a := js.Global().Get("Uint8Array").New(len(s) * 8)
		js.CopyBytesToJS(a, sliceToByteSlice(s))
		runtime.KeepAlive(s)
		buf := a.Get("buffer")
		return js.Global().Get("Float64Array").New(buf, a.Get("byteOffset"), a.Get("byteLength").Int()/8)
	default:
		panic("jsutil: unexpected value at SliceToTypedArray: " + js.ValueOf(s).Type().String())
	}
}
