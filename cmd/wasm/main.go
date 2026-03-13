//go:build js && wasm

package main

import (
	"strconv"
	"syscall/js"
	"time"

	"github.com/go-gl/mathgl/mgl32"
)

// ── Attractor state ──────────────────────────────────────────────────────────

var (
	x, y, z float32 = 0.1, 0.5, -0.6
	steps   int     = 20000
	vertBuf = make([]float32, 20000*3) // pre-allocated vertex buffer for attractors
)

// Persistent JS typed arrays — allocated once, reused every frame to avoid GC pressure.
var (
	jsVertUint8  js.Value // Uint8Array for CopyBytesToJS
	jsVertFloat  js.Value // Float32Array view for bufferData
)

// ── Camera / view state ──────────────────────────────────────────────────────

var (
	initCameraDist                     float32 = 100
	defaultCameraDist                  float32 = 100
	cameraDist                         float32
	rotationX, rotationY, rotationZ    float32
	rotationX1, rotationY1, rotationZ1 float32
	movMatrix                          mgl32.Mat4
	tmark                              float32
	totalelapsed                       float32
)

// ── Color state ──────────────────────────────────────────────────────────────

var (
	baseColor = [3]float32{1.0, 0.0, 0.0}
	topColor  = [3]float32{0.0, 0.0, 1.0}
	midColor  = [3]float32{0.0, 1.0, 0.0}
)

// ── Selection ────────────────────────────────────────────────────────────────

var selectedMode string

// ── WebGL state ──────────────────────────────────────────────────────────────

var (
	doc      js.Value = js.Global().Get("document")
	body     js.Value = doc.Get("body")
	canvasEl js.Value = doc.Call("getElementById", "gocanvas")
	width    int      = doc.Get("body").Get("clientWidth").Int()
	height   int      = doc.Get("body").Get("clientHeight").Int()
	gl       js.Value = canvasEl.Call("getContext", "webgl")

	shaderProgram         js.Value = gl.Call("createProgram")
	attractorVertexBuffer js.Value = gl.Call("createBuffer")
	attractorIndexBuffer  js.Value = gl.Call("createBuffer")
	attractorVertices     []float32
	attractorIndices      []uint16

	glTypes GLTypes
)

// ── DOM element refs ─────────────────────────────────────────────────────────

var (
	rtc               js.Value
	cameraControl     js.Value
	rotationControlsX js.Value
	rotationControlsY js.Value
	rotationControlsZ js.Value
	sliderZoom        js.Value
	sliderX           js.Value
	sliderY           js.Value
	sliderZ           js.Value
	uBaseColorLoc     js.Value
	uTopColorLoc      js.Value
	uMidColorLoc      js.Value
	uMinZLoc          js.Value
	uMaxZLoc          js.Value
	uMinXLoc          js.Value
	uMaxXLoc          js.Value
	uGradientModeLoc  js.Value
	shadersReady      bool
	renderFrame       js.Func
)

// ── Parameter definitions with slider ranges ─────────────────────────────────

type paramDef struct {
	ID    string
	Label string
	Value *float32
	Def   float32
	Min   float32
	Max   float32
	Step  float32
}

var attractorParams = map[string][]paramDef{
	"lorenz": {
		{"lorenz-dt", "dt", &lorenzDT, 0.005, 0.001, 0.05, 0.001},
		{"lorenz-s", "σ", &lorenzS, 10.0, 1, 30, 0.1},
		{"lorenz-r", "ρ", &lorenzR, 28.0, 1, 60, 0.1},
		{"lorenz-b", "β", &lorenzB, 2.7, 0.1, 10, 0.1},
	},
	"rossler": {
		{"rossler-dt", "dt", &rosslerDT, 0.005, 0.001, 0.05, 0.001},
		{"rossler-a", "a", &rosslerA, 0.2, 0.01, 1, 0.01},
		{"rossler-b", "b", &rosslerB, 0.2, 0.01, 1, 0.01},
		{"rossler-c", "c", &rosslerC, 5.7, 1, 20, 0.1},
	},
	"chua": {
		{"chua-dt", "dt", &chuaDT, 0.005, 0.001, 0.05, 0.001},
		{"chua-a", "a", &chuaA, 40, 1, 80, 0.1},
		{"chua-b", "b", &chuaB, 3.0, 0.1, 10, 0.1},
		{"chua-c", "c", &chuaC, 28.0, 1, 60, 0.1},
	},
	"aizawa": {
		{"aizawa-dt", "dt", &aizawaDT, 0.0052, 0.001, 0.02, 0.0001},
		{"aizawa-a", "a", &aizawaA, 0.95, 0.1, 2, 0.01},
		{"aizawa-b", "b", &aizawaB, 0.7, 0.1, 2, 0.01},
		{"aizawa-c", "c", &aizawaC, 0.6, 0.1, 2, 0.01},
		{"aizawa-d", "d", &aizawaD, 3.5, 0.1, 8, 0.01},
		{"aizawa-e", "e", &aizawaE, 0.25, 0.01, 1, 0.01},
		{"aizawa-f", "f", &aizawaF, 0.1, 0.01, 1, 0.01},
	},
	"sprott": {
		{"sprott-dt", "dt", &sprottDT, 0.01, 0.001, 0.05, 0.001},
		{"sprott-a", "a", &sprottA, 2.07, 0.1, 5, 0.01},
		{"sprott-b", "b", &sprottB, 1.8, 0.1, 5, 0.01},
	},
	"lissajou": {
		{"lissajou-a", "a", &lissajouA, 9, 1, 30, 0.1},
		{"lissajou-b", "b", &lissajouB, 4, 1, 30, 0.1},
		{"lissajou-c", "c", &lissajouC, 25, 1, 50, 0.1},
	},
	"sphere": {
		{"sphere-r", "radius", &sphereRadius, 1.0, 0.1, 5, 0.1},
	},
	"torus": {
		{"torus-R", "R", &torusR, 1.5, 0.1, 5, 0.1},
		{"torus-r", "r", &torusr, 0.5, 0.1, 3, 0.1},
	},
}

// ── Controls panel HTML ──────────────────────────────────────────────────────

const controlsHTML = `
<style>
.rst{background:none;border:none;color:#666;cursor:pointer;font-size:13px;padding:0 2px;font-family:monospace;}
.rst:hover{color:#fff;}
</style>
<div style="margin-bottom:6px;">
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="rossler" checked> Rossler</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="lorenz"> Lorenz</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="chua"> Chua</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="aizawa"> Aizawa</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="sprott"> Sprott</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="lissajou"> Lissajou</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="cube"> Cube</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="nestedcube"> Nested Cube</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="sphere"> Sphere</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="torus"> Torus</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="magnetosphere"> Magnetosphere</label>
  <button id="reset-all-btn" style="margin-left:8px;background:#333;color:#ccc;border:1px solid #555;padding:2px 8px;cursor:pointer;font-family:monospace;font-size:12px;">Reset All</button>
</div>
<div id="params" style="margin-bottom:6px;"></div>
<div style="margin-bottom:4px;">
  <label>Base <input type="color" id="color-base" value="#ff0000"></label>
  <button class="rst" id="rst-color-base" title="Reset">↺</button>
  <label style="margin-left:4px;">Mid <input type="color" id="color-mid" value="#00ff00"></label>
  <button class="rst" id="rst-color-mid" title="Reset">↺</button>
  <label style="margin-left:4px;">Top <input type="color" id="color-top" value="#0000ff"></label>
  <button class="rst" id="rst-color-top" title="Reset">↺</button>
  <span style="margin-left:8px;">
  <label>Zoom</label>
  <input type="range" id="camera-zoom" min="-1" max="1" value="0" step="0.1" style="width:80px;vertical-align:middle;">
  <output id="slider-value-zoom" style="margin-right:2px;width:24px;display:inline-block;">0</output>
  <button class="rst" id="rst-zoom" title="Reset">↺</button>
  <label style="margin-left:4px;">X</label>
  <input type="range" id="rotation-controls-x" min="-1" max="1" value="0" step="0.1" style="width:80px;vertical-align:middle;">
  <output id="slider-value-x" style="margin-right:2px;width:24px;display:inline-block;">0</output>
  <button class="rst" id="rst-rx" title="Reset">↺</button>
  <label style="margin-left:4px;">Y</label>
  <input type="range" id="rotation-controls-y" min="-1" max="1" value="0" step="0.1" style="width:80px;vertical-align:middle;">
  <output id="slider-value-y" style="margin-right:2px;width:24px;display:inline-block;">0</output>
  <button class="rst" id="rst-ry" title="Reset">↺</button>
  <label style="margin-left:4px;">Z</label>
  <input type="range" id="rotation-controls-z" min="-1" max="1" value="0" step="0.1" style="width:80px;vertical-align:middle;">
  <output id="slider-value-z" style="margin-right:2px;width:24px;display:inline-block;">0</output>
  <button class="rst" id="rst-rz" title="Reset">↺</button>
  </span>
</div>
<div id="runtime" style="color:#555;font-size:11px;"></div>
`

// ── init ─────────────────────────────────────────────────────────────────────

func init() {
	gl = canvasEl.Call("getContext", "webgl")
	canvasEl.Set("width", width)
	canvasEl.Set("height", height)
	if gl.IsUndefined() {
		gl = canvasEl.Call("getContext", "experimental-webgl")
	}
	if gl.IsUndefined() {
		js.Global().Call("alert", "browser might not support webgl")
		return
	}
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	if body.IsUndefined() {
		body = doc.Get("body")
	}
	if body.IsUndefined() {
		js.Global().Call("alert", "cannot get html body, exiting")
		return
	}

	// Build controls panel
	panel := doc.Call("createElement", "div")
	panel.Set("id", "controls-panel")
	panel.Set("style", "position:fixed;top:0;left:0;right:0;z-index:10;background:rgba(0,0,0,0.85);padding:8px 12px;font-family:monospace;font-size:12px;color:#aaa;border-bottom:1px solid #333;pointer-events:auto;")
	panel.Set("innerHTML", controlsHTML)
	body.Call("appendChild", panel)

	// Refresh DOM
	doc = js.Global().Get("document")
	body = doc.Get("body")

	// Get control element references
	rtc = doc.Call("getElementById", "runtime")
	cameraControl = doc.Call("getElementById", "camera-zoom")
	rotationControlsX = doc.Call("getElementById", "rotation-controls-x")
	rotationControlsY = doc.Call("getElementById", "rotation-controls-y")
	rotationControlsZ = doc.Call("getElementById", "rotation-controls-z")
	sliderZoom = doc.Call("getElementById", "slider-value-zoom")
	sliderX = doc.Call("getElementById", "slider-value-x")
	sliderY = doc.Call("getElementById", "slider-value-y")
	sliderZ = doc.Call("getElementById", "slider-value-z")

	// Event: mode change
	radios := doc.Call("querySelectorAll", `input[name="mode"]`)
	modeCallback := js.FuncOf(onModeChange)
	for i := 0; i < radios.Length(); i++ {
		radios.Index(i).Call("addEventListener", "change", modeCallback)
	}

	// Event: color pickers
	colorCallback := js.FuncOf(onColorChange)
	doc.Call("getElementById", "color-base").Call("addEventListener", "input", colorCallback)
	doc.Call("getElementById", "color-mid").Call("addEventListener", "input", colorCallback)
	doc.Call("getElementById", "color-top").Call("addEventListener", "input", colorCallback)

	// Event: per-control reset buttons for colors
	doc.Call("getElementById", "rst-color-base").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		baseColor = [3]float32{1.0, 0.0, 0.0}
		doc.Call("getElementById", "color-base").Set("value", "#ff0000")
		gl.Call("uniform3f", uBaseColorLoc, baseColor[0], baseColor[1], baseColor[2])
		return nil
	}))
	doc.Call("getElementById", "rst-color-mid").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		midColor = [3]float32{0.0, 1.0, 0.0}
		doc.Call("getElementById", "color-mid").Set("value", "#00ff00")
		gl.Call("uniform3f", uMidColorLoc, midColor[0], midColor[1], midColor[2])
		return nil
	}))
	doc.Call("getElementById", "rst-color-top").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		topColor = [3]float32{0.0, 0.0, 1.0}
		doc.Call("getElementById", "color-top").Set("value", "#0000ff")
		gl.Call("uniform3f", uTopColorLoc, topColor[0], topColor[1], topColor[2])
		return nil
	}))

	// Event: per-control reset buttons for camera/rotation
	doc.Call("getElementById", "rst-zoom").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		defaultCameraDist = initCameraDist
		cameraDist = 0
		cameraControl.Set("value", "0")
		sliderZoom.Set("textContent", "0.0")
		updateViewMatrix()
		return nil
	}))
	doc.Call("getElementById", "rst-rx").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		rotationX, rotationX1 = 0, 0
		rotationControlsX.Set("value", "0")
		sliderX.Set("textContent", "0.0")
		return nil
	}))
	doc.Call("getElementById", "rst-ry").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		rotationY, rotationY1 = 0, 0
		rotationControlsY.Set("value", "0")
		sliderY.Set("textContent", "0.0")
		return nil
	}))
	doc.Call("getElementById", "rst-rz").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		rotationZ, rotationZ1 = 0, 0
		rotationControlsZ.Set("value", "0")
		sliderZ.Set("textContent", "0.0")
		return nil
	}))

	// Event: reset all button
	doc.Call("getElementById", "reset-all-btn").Call("addEventListener", "click", js.FuncOf(onResetAll))

	// Initial mode
	selectedMode = "rossler"
	buildParamPanel(selectedMode)

	// Initialize persistent JS typed arrays for zero-alloc frame uploads
	jsVertUint8 = js.Global().Get("Uint8Array").New(steps * 3 * 4)
	buf := jsVertUint8.Get("buffer")
	jsVertFloat = js.Global().Get("Float32Array").New(buf, 0, steps*3)

	// Initialize WebGL
	glTypes.New(gl)
	// Bind buffers before setting up attrib pointers in setupShaders
	gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
	gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
	setupShaders()
	setupMatrices()
	generateForMode(selectedMode)
	refreshGradient()

	// Start animation loop
	done := make(chan struct{})
	renderFrame = js.FuncOf(renderLoop)
	js.Global().Call("requestAnimationFrame", renderFrame)

	// Clock goroutine
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if !rtc.IsUndefined() {
				rtc.Set("innerHTML", time.Now().Format("2006-01-02 15:04:05"))
			}
		}
	}()

	<-done
}

// ── UI helpers ───────────────────────────────────────────────────────────────

func buildParamPanel(mode string) {
	paramsDiv := doc.Call("getElementById", "params")
	paramsDiv.Set("innerHTML", "")

	params, ok := attractorParams[mode]
	if !ok || len(params) == 0 {
		return
	}

	for _, p := range params {
		p := p // capture for closure

		span := doc.Call("createElement", "span")
		span.Set("style", "margin-right:10px;color:#888;display:inline-block;")

		lbl := doc.Call("createElement", "span")
		lbl.Set("textContent", p.Label+" ")
		span.Call("appendChild", lbl)

		input := doc.Call("createElement", "input")
		input.Set("type", "range")
		input.Set("id", p.ID)
		input.Set("min", strconv.FormatFloat(float64(p.Min), 'g', -1, 32))
		input.Set("max", strconv.FormatFloat(float64(p.Max), 'g', -1, 32))
		input.Set("value", strconv.FormatFloat(float64(*p.Value), 'g', -1, 32))
		input.Set("step", strconv.FormatFloat(float64(p.Step), 'g', -1, 32))
		input.Set("style", "width:80px;vertical-align:middle;")

		output := doc.Call("createElement", "output")
		output.Set("style", "width:40px;display:inline-block;font-size:11px;text-align:left;margin-left:2px;")
		output.Set("textContent", strconv.FormatFloat(float64(*p.Value), 'g', -1, 32))

		input.Call("addEventListener", "input", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			val, err := strconv.ParseFloat(input.Get("value").String(), 32)
			if err == nil {
				*p.Value = float32(val)
				output.Set("textContent", strconv.FormatFloat(val, 'g', -1, 32))
				x, y, z = 0.1, 0.5, -0.6
				refreshGradient()
			}
			return nil
		}))

		// Per-param reset button
		rst := doc.Call("createElement", "button")
		rst.Set("className", "rst")
		rst.Set("title", "Reset "+p.Label)
		rst.Set("textContent", "↺")
		rst.Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			*p.Value = p.Def
			defStr := strconv.FormatFloat(float64(p.Def), 'g', -1, 32)
			input.Set("value", defStr)
			output.Set("textContent", defStr)
			x, y, z = 0.1, 0.5, -0.6
			refreshGradient()
			return nil
		}))

		span.Call("appendChild", input)
		span.Call("appendChild", output)
		span.Call("appendChild", rst)
		paramsDiv.Call("appendChild", span)
	}
}

func hexToRGB(hex string) (float32, float32, float32) {
	if len(hex) < 7 {
		return 1, 1, 1
	}
	r, _ := strconv.ParseInt(hex[1:3], 16, 64)
	g, _ := strconv.ParseInt(hex[3:5], 16, 64)
	b, _ := strconv.ParseInt(hex[5:7], 16, 64)
	return float32(r) / 255.0, float32(g) / 255.0, float32(b) / 255.0
}

// ── Event handlers ───────────────────────────────────────────────────────────

func refreshGradient() {
	if shadersReady && len(attractorVertices) > 0 {
		updateGradientRange(attractorVertices)
	}
}

func onModeChange(this js.Value, args []js.Value) interface{} {
	selectedMode = doc.Call("querySelector", `input[name="mode"]:checked`).Get("value").String()
	x, y, z = 0.1, 0.5, -0.6
	buildParamPanel(selectedMode)
	// Run one frame to populate vertices, then update gradient
	generateForMode(selectedMode)
	refreshGradient()
	return nil
}

func onColorChange(this js.Value, args []js.Value) interface{} {
	baseHex := doc.Call("getElementById", "color-base").Get("value").String()
	midHex := doc.Call("getElementById", "color-mid").Get("value").String()
	topHex := doc.Call("getElementById", "color-top").Get("value").String()
	baseColor[0], baseColor[1], baseColor[2] = hexToRGB(baseHex)
	midColor[0], midColor[1], midColor[2] = hexToRGB(midHex)
	topColor[0], topColor[1], topColor[2] = hexToRGB(topHex)
	gl.Call("uniform3f", uBaseColorLoc, baseColor[0], baseColor[1], baseColor[2])
	gl.Call("uniform3f", uMidColorLoc, midColor[0], midColor[1], midColor[2])
	gl.Call("uniform3f", uTopColorLoc, topColor[0], topColor[1], topColor[2])
	return nil
}

func onResetAll(this js.Value, args []js.Value) interface{} {
	// Reset camera
	defaultCameraDist = initCameraDist
	cameraDist = 0
	rotationX, rotationY, rotationZ = 0, 0, 0
	rotationX1, rotationY1, rotationZ1 = 0, 0, 0
	movMatrix = mgl32.Ident4()

	// Reset attractor position
	x, y, z = 0.1, 0.5, -0.6

	// Reset sliders
	cameraControl.Set("value", "0")
	rotationControlsX.Set("value", "0")
	rotationControlsY.Set("value", "0")
	rotationControlsZ.Set("value", "0")
	sliderZoom.Set("textContent", "0.0")
	sliderX.Set("textContent", "0.0")
	sliderY.Set("textContent", "0.0")
	sliderZ.Set("textContent", "0.0")

	// Reset all parameters to defaults
	for _, params := range attractorParams {
		for _, p := range params {
			*p.Value = p.Def
		}
	}
	buildParamPanel(selectedMode)

	// Reset colors
	baseColor = [3]float32{1.0, 0.0, 0.0}
	midColor = [3]float32{0.0, 1.0, 0.0}
	topColor = [3]float32{0.0, 0.0, 1.0}
	doc.Call("getElementById", "color-base").Set("value", "#ff0000")
	doc.Call("getElementById", "color-mid").Set("value", "#00ff00")
	doc.Call("getElementById", "color-top").Set("value", "#0000ff")
	gl.Call("uniform3f", uBaseColorLoc, baseColor[0], baseColor[1], baseColor[2])
	gl.Call("uniform3f", uMidColorLoc, midColor[0], midColor[1], midColor[2])
	gl.Call("uniform3f", uTopColorLoc, topColor[0], topColor[1], topColor[2])

	// Reset view
	generateForMode(selectedMode)
	updateViewMatrix()
	updateModelMatrix()

	return nil
}
