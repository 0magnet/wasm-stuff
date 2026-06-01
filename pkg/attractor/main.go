//go:build js && wasm

package attractor

import (
	"fmt"
	"math"
	"runtime"
	"strconv"
	"syscall/js"
	"time"

	"github.com/go-gl/mathgl/mgl32"
)

// ── Debug stats ─────────────────────────────────────────────────────────────

var (
	debugEnabled   bool
	frameCount     int
	frameTotalMs   float32
	frameMinMs     float32 = 999
	frameMaxMs     float32
	lastFrameStart float32
)

// ── Attractor state ──────────────────────────────────────────────────────────

var (
	x, y, z   float32 = 0.1, 0.5, -0.6
	steps      int = 20000
	vertBuf        = make([]float32, 20000*4) // pre-allocated vertex buffer (stride 4: x,y,z,t)
	speedSteps int     = 1
	speedScale float32 = 1.0 // dt multiplier for sub-1 speeds
	// centerOffset is computed after warmup frames and then held stable
	centerOffset [3]float32
	centerReady  bool
	centerWarmup int
)

// attractorDrawMode is the GL draw mode (LineStrip or Points) — set after glTypes.New.
var attractorDrawMode js.Value

// Persistent JS typed arrays — allocated once, reused every frame to avoid GC pressure.
var (
	jsVertUint8  js.Value // Uint8Array for CopyBytesToJS
	jsVertFloat  js.Value // Float32Array view for bufferData
)

// ── Camera / view state ──────────────────────────────────────────────────────

var (
	initCameraDist    float32 = 100
	defaultCameraDist float32 = 100
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
	bgColor   = [3]float32{0.0, 0.0, 0.0}
)

// ── Interaction state ────────────────────────────────────────────────────────

var (
	paused          bool    = false
	stopped         bool    = false
	pausedCount     int     = 0
	autoRotate      bool    = true
	autoRotateSpeed float32 = 0.005
	usePoints       bool    = false
	persistTrail    bool    = false
	gradientMode    int     = 0 // current gradient mode uniform value
	gradientReverse bool    = false
	dragging        bool    = false
	dragLastX       float32
	dragLastY       float32
	dragRotX        float32
	dragRotY        float32
)

// ── Selection ────────────────────────────────────────────────────────────────

var selectedMode string

// ── WebGL state ──────────────────────────────────────────────────────────────

// These are package-level for convenience (lots of helper functions
// across the package reach for them), but they MUST NOT be initialized
// at package-var time. When attractor is imported as a library (e.g.
// from m2/wasm/stl2), package-var init runs before the host's DOM is
// ready, so getElementById("gocanvas") returns null and the subsequent
// canvasEl.Call("getContext", "webgl") panics with
// "syscall/js: call of Value.Call on null". initWebGL(), called from
// Run() once the canvas exists, populates them.
var (
	doc      js.Value
	body     js.Value
	canvasEl js.Value
	width    int
	height   int
	gl       js.Value

	shaderProgram         js.Value
	attractorVertexBuffer js.Value
	attractorIndexBuffer  js.Value
	attractorVertices     []float32
	attractorIndices      []uint16

	glTypes GLTypes
)

func initWebGL() {
	doc = js.Global().Get("document")
	body = doc.Get("body")
	canvasEl = doc.Call("getElementById", "gocanvas")
	if canvasEl.IsUndefined() || canvasEl.IsNull() {
		return
	}
	width = doc.Get("body").Get("clientWidth").Int()
	height = doc.Get("body").Get("clientHeight").Int()
	opts := js.Global().Get("Object").New()
	opts.Set("preserveDrawingBuffer", true)
	gl = canvasEl.Call("getContext", "webgl", opts)
	canvasEl.Set("width", width)
	canvasEl.Set("height", height)
	if gl.IsUndefined() {
		gl = canvasEl.Call("getContext", "experimental-webgl", opts)
	}
	if gl.IsUndefined() {
		js.Global().Call("alert", "browser might not support webgl")
		return
	}
	shaderProgram = gl.Call("createProgram")
	attractorVertexBuffer = gl.Call("createBuffer")
	attractorIndexBuffer = gl.Call("createBuffer")
}

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
	uMinXLoc            js.Value
	uMaxXLoc            js.Value
	uMinYLoc            js.Value
	uMaxYLoc            js.Value
	uGradientModeLoc    js.Value
	uGradientReverseLoc js.Value
	positionLoc         js.Value
	aTrailTLoc          js.Value
	shadersReady        bool
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
		{"chua-alpha", "α", &chuaAlpha, 15.6, 5, 30, 0.1},
		{"chua-beta", "β", &chuaBeta, 28.0, 10, 50, 0.1},
		{"chua-m0", "m0", &chuaM0, -1.143, -2, 0, 0.001},
		{"chua-m1", "m1", &chuaM1, -0.714, -2, 0, 0.001},
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
		{"sprott-b", "b", &sprottB, 1.79, 0.1, 5, 0.01},
	},
	"lissajou": {
		{"lissajou-a", "a", &lissajouA, 3, 1, 20, 1},
		{"lissajou-b", "b", &lissajouB, 2, 1, 20, 1},
		{"lissajou-c", "c", &lissajouC, 5, 1, 20, 1},
	},
	"thomas": {
		{"thomas-dt", "dt", &thomasDT, 0.05, 0.001, 0.1, 0.001},
		{"thomas-b", "b", &thomasB, 0.208186, 0.01, 1.0, 0.001},
	},
	"halvorsen": {
		{"halvorsen-dt", "dt", &halvorsenDT, 0.005, 0.001, 0.05, 0.001},
		{"halvorsen-a", "a", &halvorsenA, 1.89, 0.1, 5, 0.01},
	},
	"chen": {
		{"chen-dt", "dt", &chenDT, 0.0005, 0.0001, 0.005, 0.0001},
		{"chen-a", "a", &chenA, 35.0, 10, 50, 0.1},
		{"chen-b", "b", &chenB, 3.0, 0.1, 10, 0.1},
		{"chen-c", "c", &chenC, 28.0, 10, 40, 0.1},
	},
	"dadras": {
		{"dadras-dt", "dt", &dadrasDT, 0.005, 0.001, 0.05, 0.001},
		{"dadras-p", "p", &dadrasP, 3.0, 0.1, 10, 0.1},
		{"dadras-q", "q", &dadrasQ, 2.7, 0.1, 10, 0.1},
		{"dadras-r", "r", &dadrasR, 1.7, 0.1, 10, 0.1},
		{"dadras-s", "s", &dadrasS, 2.0, 0.1, 10, 0.1},
		{"dadras-e", "e", &dadrasE, 9.0, 0.1, 20, 0.1},
	},
	"rabinovich": {
		{"rab-dt", "dt", &rabDT, 0.001, 0.0001, 0.01, 0.0001},
		{"rab-alpha", "α", &rabAlpha, 0.14, 0.01, 1, 0.01},
		{"rab-gamma", "γ", &rabGamma, 0.10, 0.01, 1, 0.01},
	},
	"burkeshaw": {
		{"burke-dt", "dt", &burkeDT, 0.005, 0.001, 0.05, 0.001},
		{"burke-s", "S", &burkeS, 10.0, 1, 20, 0.1},
		{"burke-v", "V", &burkeV, 4.272, 1, 10, 0.001},
	},
	"globe": {
		{"globe-lat", "lat", &globeLatF, 18, 4, 90, 1},
		{"globe-lon", "lon", &globeLonF, 36, 4, 180, 1},
	},
	"sphere": {
		{"sphere-r", "radius", &sphereRadius, 1.0, 0.1, 5, 0.1},
		{"sphere-stacks", "lat", &sphereStacksF, 30, 4, 100, 1},
		{"sphere-slices", "lon", &sphereSlicesF, 30, 4, 100, 1},
	},
	"torus": {
		{"torus-R", "R", &torusR, 1.5, 0.1, 5, 0.1},
		{"torus-r", "r", &torusr, 0.5, 0.1, 3, 0.1},
		{"torus-stacks", "stacks", &torusStacksF, 30, 4, 100, 1},
		{"torus-slices", "slices", &torusSlicesF, 30, 4, 100, 1},
	},
}

// ── Controls panel HTML ──────────────────────────────────────────────────────

const controlsHTML = `
<style>
.rst{background:none;border:none;color:#666;cursor:pointer;font-size:13px;padding:0 2px;font-family:monospace;}
.rst:hover{color:#fff;}
.ctrl-btn{background:#333;color:#ccc;border:1px solid #555;padding:2px 8px;cursor:pointer;font-family:monospace;font-size:12px;margin-left:4px;}
.ctrl-btn:hover{background:#555;}
</style>
<div style="margin-bottom:6px;line-height:1.8;">
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="rossler"> Rossler</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="lorenz"> Lorenz</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="chua"> Chua</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="aizawa"> Aizawa</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="sprott"> Sprott</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="lissajou"> Lissajou</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="thomas"> Thomas</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="halvorsen"> Halvorsen</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="chen"> Chen</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="dadras"> Dadras</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="rabinovich"> Rabinovich-Fabrikant</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="burkeshaw"> Burke-Shaw</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="tetrahedron"> Tetrahedron</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="cube"> Cube</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="octahedron"> Octahedron</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="dodecahedron"> Dodecahedron</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="icosahedron"> Icosahedron</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="nestedcube"> Nested Cube</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="globe" checked> Globe</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="sphere"> Sphere</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="torus"> Torus</label>
  <label style="margin-right:4px;cursor:pointer;"><input type="radio" name="mode" value="magnetosphere"> Magnetosphere</label>
  <button id="reset-all-btn" class="ctrl-btn">Reset All</button>
</div>
<div id="params" style="margin-bottom:6px;"></div>
<div style="margin-bottom:4px;">
  <label>Base <input type="color" id="color-base" value="#ff0000"></label>
  <button class="rst" id="rst-color-base" title="Reset">↺</button>
  <label style="margin-left:4px;">Mid <input type="color" id="color-mid" value="#00ff00"></label>
  <button class="rst" id="rst-color-mid" title="Reset">↺</button>
  <label style="margin-left:4px;">Top <input type="color" id="color-top" value="#0000ff"></label>
  <button class="rst" id="rst-color-top" title="Reset">↺</button>
  <label style="margin-left:8px;">BG <input type="color" id="color-bg" value="#000000"></label>
  <button class="rst" id="rst-color-bg" title="Reset">↺</button>
  <label style="margin-left:8px;">Gradient
  <select id="gradient-type" style="background:#222;color:#ccc;border:1px solid #555;font-family:monospace;font-size:11px;">
    <option value="z2">Z Two-color</option>
    <option value="x3">X Three-color</option>
    <option value="y2">Y Two-color</option>
    <option value="x2">X Two-color</option>
    <option value="trail-rainbow">Trail Rainbow</option>
    <option value="trail2">Trail Two-color</option>
    <option value="trail3">Trail Three-color</option>
    <option value="z-rainbow">Z Rainbow</option>
    <option value="x-rainbow">X Rainbow</option>
    <option value="y-rainbow">Y Rainbow</option>
  </select></label>
  <label style="margin-left:4px;cursor:pointer;"><input type="checkbox" id="gradient-reverse"> Reverse</label>
  <span style="margin-left:8px;">
  <label>Zoom</label>
  <input type="range" id="camera-zoom" min="-95" max="95" value="0" step="1" style="width:120px;vertical-align:middle;">
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
<div style="margin-bottom:4px;">
  <label>Speed <input type="range" id="speed-slider" min="-2" max="2" value="0" step="0.01" style="width:80px;vertical-align:middle;"></label>
  <output id="slider-value-speed" style="width:40px;display:inline-block;">1</output>
  <button class="rst" id="rst-speed" title="Reset">↺</button>
  <label style="margin-left:8px;cursor:pointer;"><input type="checkbox" id="auto-rotate" checked> Auto-rotate</label>
  <label style="margin-left:8px;cursor:pointer;"><input type="checkbox" id="use-points"> Points</label>
  <label style="margin-left:8px;">Trail <input type="range" id="trail-slider" min="1000" max="500000" value="20000" step="1000" style="width:100px;vertical-align:middle;"></label>
  <output id="slider-value-trail" style="width:50px;display:inline-block;">20000</output>
  <button class="rst" id="rst-trail" title="Reset">↺</button>
  <label style="margin-left:4px;cursor:pointer;"><input type="checkbox" id="persist-trail"> Persist</label>
  <label style="margin-left:8px;cursor:pointer;"><input type="checkbox" id="show-info"> Info</label>
  <button id="pause-btn" class="ctrl-btn">Pause</button>
  <button id="stop-btn" class="ctrl-btn" style="background:#c00;color:#fff;border-color:#900;font-weight:bold;">Stop Rendering</button>
  <button id="fullscreen-btn" class="ctrl-btn">Fullscreen</button>
  <button id="screenshot-btn" class="ctrl-btn">Screenshot</button>
</div>
<div id="runtime" style="color:#555;font-size:11px;"></div>
`

// ── main ─────────────────────────────────────────────────────────────────────

func Run() {
	// Lazy WebGL init — see initWebGL doc. Must run after the host
	// DOM is ready (caller's responsibility); otherwise gocanvas
	// won't exist yet and canvasEl.Call("getContext", ...) panics.
	initWebGL()
	if body.IsUndefined() || body.IsNull() {
		js.Global().Call("alert", "cannot get html body, exiting")
		return
	}
	if canvasEl.IsUndefined() || canvasEl.IsNull() {
		js.Global().Call("alert", "cannot find #gocanvas, exiting")
		return
	}

	// Build controls panel
	panel := doc.Call("createElement", "div")
	panel.Set("id", "controls-panel")
	panel.Set("style", "position:fixed;bottom:0;left:0;right:0;z-index:10;background:rgba(0,0,0,0.85);padding:8px 12px;font-family:monospace;font-size:12px;color:#aaa;border-top:1px solid #333;pointer-events:auto;max-height:50vh;overflow-y:auto;")
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
		cameraControl.Set("value", "0")
		sliderZoom.Set("textContent", "0")
		updateViewMatrix()
		return nil
	}))
	stopAutoRotate := func() {
		autoRotate = false
		doc.Call("getElementById", "auto-rotate").Set("checked", false)
	}
	doc.Call("getElementById", "rst-rx").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		rotationX, rotationX1 = 0, 0
		rotationControlsX.Set("value", "0")
		sliderX.Set("textContent", "0.0")
		stopAutoRotate()
		return nil
	}))
	doc.Call("getElementById", "rst-ry").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		rotationY, rotationY1 = 0, 0
		rotationControlsY.Set("value", "0")
		sliderY.Set("textContent", "0.0")
		stopAutoRotate()
		return nil
	}))
	doc.Call("getElementById", "rst-rz").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		rotationZ, rotationZ1 = 0, 0
		rotationControlsZ.Set("value", "0")
		sliderZ.Set("textContent", "0.0")
		stopAutoRotate()
		return nil
	}))

	// Event: reset all button
	doc.Call("getElementById", "reset-all-btn").Call("addEventListener", "click", js.FuncOf(onResetAll))

	// Event: speed slider
	doc.Call("getElementById", "speed-slider").Call("addEventListener", "input", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		val, err := strconv.ParseFloat(doc.Call("getElementById", "speed-slider").Get("value").String(), 64)
		if err == nil {
			applySpeedLog(val)
		}
		return nil
	}))
	doc.Call("getElementById", "rst-speed").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		speedSteps = 1
		speedScale = 1.0
		doc.Call("getElementById", "speed-slider").Set("value", "0")
		doc.Call("getElementById", "slider-value-speed").Set("textContent", "1")
		return nil
	}))

	// Event: auto-rotate checkbox
	doc.Call("getElementById", "auto-rotate").Call("addEventListener", "change", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		autoRotate = doc.Call("getElementById", "auto-rotate").Get("checked").Bool()
		return nil
	}))

	// Event: points/line toggle
	doc.Call("getElementById", "use-points").Call("addEventListener", "change", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		usePoints = doc.Call("getElementById", "use-points").Get("checked").Bool()
		if usePoints {
			attractorDrawMode = glTypes.Points
		} else {
			attractorDrawMode = glTypes.LineStrip
		}
		return nil
	}))

	// Event: trail length slider
	doc.Call("getElementById", "trail-slider").Call("addEventListener", "input", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		val, err := strconv.ParseFloat(doc.Call("getElementById", "trail-slider").Get("value").String(), 64)
		if err == nil {
			newSteps := int(val)
			if newSteps != steps {
				steps = newSteps
				vertBuf = make([]float32, steps*4)
				jsVertUint8 = js.Global().Get("Uint8Array").New(steps * 4 * 4)
				buf := jsVertUint8.Get("buffer")
				jsVertFloat = js.Global().Get("Float32Array").New(buf, 0, steps*4)
				resetAttractorState()
				refreshGradient()
			}
			doc.Call("getElementById", "slider-value-trail").Set("textContent", strconv.FormatFloat(val, 'f', 0, 64))
		}
		return nil
	}))
	doc.Call("getElementById", "rst-trail").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		steps = 20000
		vertBuf = make([]float32, steps*4)
		jsVertUint8 = js.Global().Get("Uint8Array").New(steps * 4 * 4)
		buf := jsVertUint8.Get("buffer")
		jsVertFloat = js.Global().Get("Float32Array").New(buf, 0, steps*4)
		doc.Call("getElementById", "trail-slider").Set("value", "20000")
		doc.Call("getElementById", "slider-value-trail").Set("textContent", "20000")
		persistTrail = false
		doc.Call("getElementById", "persist-trail").Set("checked", false)
		resetAttractorState()
		refreshGradient()
		return nil
	}))

	// Event: persist trail checkbox
	doc.Call("getElementById", "persist-trail").Call("addEventListener", "change", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		persistTrail = doc.Call("getElementById", "persist-trail").Get("checked").Bool()
		return nil
	}))

	// Create info overlay div
	infoOverlay := doc.Call("createElement", "div")
	infoOverlay.Set("id", "info-overlay")
	infoOverlay.Set("style", "display:none;position:fixed;top:140px;left:20px;right:20px;z-index:15;"+
		"color:rgba(255,255,255,0.85);font-family:monospace;font-size:14px;line-height:1.6;"+
		"white-space:pre-wrap;pointer-events:none;text-shadow:0 0 10px #000,0 0 20px #000;"+
		"max-width:600px;")
	body.Call("appendChild", infoOverlay)

	// Event: show info checkbox
	doc.Call("getElementById", "show-info").Call("addEventListener", "change", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		checked := doc.Call("getElementById", "show-info").Get("checked").Bool()
		overlay := doc.Call("getElementById", "info-overlay")
		if checked {
			if desc, ok := attractorDescriptions[selectedMode]; ok {
				overlay.Set("textContent", desc)
			} else {
				overlay.Set("textContent", selectedMode)
			}
			overlay.Get("style").Set("display", "block")
		} else {
			overlay.Get("style").Set("display", "none")
		}
		return nil
	}))

	// Event: background color picker
	doc.Call("getElementById", "color-bg").Call("addEventListener", "input", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		hex := doc.Call("getElementById", "color-bg").Get("value").String()
		bgColor[0], bgColor[1], bgColor[2] = hexToRGB(hex)
		gl.Call("clearColor", bgColor[0], bgColor[1], bgColor[2], 1.0)
		return nil
	}))
	doc.Call("getElementById", "rst-color-bg").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		bgColor = [3]float32{0, 0, 0}
		doc.Call("getElementById", "color-bg").Set("value", "#000000")
		gl.Call("clearColor", 0, 0, 0, 1.0)
		return nil
	}))

	// Event: gradient type selector
	gradientTypeMap := map[string]int{
		"z2": 0, "x3": 1, "y2": 2, "x2": 3,
		"trail-rainbow": 4, "trail2": 5, "trail3": 6,
		"z-rainbow": 7, "x-rainbow": 8, "y-rainbow": 9,
	}
	doc.Call("getElementById", "gradient-type").Call("addEventListener", "change", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		val := doc.Call("getElementById", "gradient-type").Get("value").String()
		if mode, ok := gradientTypeMap[val]; ok {
			gradientMode = mode
		}
		return nil
	}))
	doc.Call("getElementById", "gradient-reverse").Call("addEventListener", "change", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		gradientReverse = doc.Call("getElementById", "gradient-reverse").Get("checked").Bool()
		return nil
	}))

	// Event: pause button
	doc.Call("getElementById", "pause-btn").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		paused = !paused
		btn := doc.Call("getElementById", "pause-btn")
		if paused {
			btn.Set("textContent", "Play")
		} else {
			btn.Set("textContent", "Pause")
		}
		return nil
	}))

	// Event: stop button — clears the canvas, kills the render loop
	// (renderLoop short-circuits on `stopped` without rescheduling),
	// and tears the controls panel + runtime overlay out of the DOM.
	// One-way: reload to restart. Matches the legacy stl2 "Stop
	// Rendering" semantics the magnetosphere.net home page had before
	// this delegation.
	doc.Call("getElementById", "stop-btn").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		stopped = true
		paused = true
		gl.Call("clearColor", 0, 0, 0, 0)
		gl.Call("clear", glTypes.ColorBufferBit)
		gl.Call("clear", glTypes.DepthBufferBit)
		panel := doc.Call("getElementById", "controls-panel")
		if !panel.IsNull() && !panel.IsUndefined() {
			parent := panel.Get("parentNode")
			if !parent.IsNull() && !parent.IsUndefined() {
				parent.Call("removeChild", panel)
			}
		}
		runtimeEl := doc.Call("getElementById", "runtime")
		if !runtimeEl.IsNull() && !runtimeEl.IsUndefined() {
			parent := runtimeEl.Get("parentNode")
			if !parent.IsNull() && !parent.IsUndefined() {
				parent.Call("removeChild", runtimeEl)
			}
		}
		return nil
	}))

	// Event: fullscreen button
	doc.Call("getElementById", "fullscreen-btn").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		docEl := doc.Get("documentElement")
		if !docEl.IsUndefined() {
			rfs := docEl.Get("requestFullscreen")
			if !rfs.IsUndefined() {
				docEl.Call("requestFullscreen")
			} else {
				wkRfs := docEl.Get("webkitRequestFullscreen")
				if !wkRfs.IsUndefined() {
					docEl.Call("webkitRequestFullscreen")
				}
			}
		}
		return nil
	}))

	// Event: screenshot button
	doc.Call("getElementById", "screenshot-btn").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		dataURL := canvasEl.Call("toDataURL", "image/png")
		link := doc.Call("createElement", "a")
		link.Set("download", "attractor.png")
		link.Set("href", dataURL)
		link.Call("click")
		return nil
	}))

	// Event: mouse drag rotation on canvas
	canvasEl.Call("addEventListener", "mousedown", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		dragging = true
		dragLastX = float32(e.Get("clientX").Float())
		dragLastY = float32(e.Get("clientY").Float())
		return nil
	}))
	js.Global().Call("addEventListener", "mousemove", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if !dragging {
			return nil
		}
		e := args[0]
		cx := float32(e.Get("clientX").Float())
		cy := float32(e.Get("clientY").Float())
		dragRotY += (cx - dragLastX) * 0.005
		dragRotX += (cy - dragLastY) * 0.005
		dragLastX = cx
		dragLastY = cy
		return nil
	}))
	js.Global().Call("addEventListener", "mouseup", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		dragging = false
		return nil
	}))

	// Event: touch drag rotation on canvas
	canvasEl.Call("addEventListener", "touchstart", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		e.Call("preventDefault")
		t := e.Get("touches").Index(0)
		dragging = true
		dragLastX = float32(t.Get("clientX").Float())
		dragLastY = float32(t.Get("clientY").Float())
		return nil
	}))
	canvasEl.Call("addEventListener", "touchmove", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if !dragging {
			return nil
		}
		e := args[0]
		e.Call("preventDefault")
		t := e.Get("touches").Index(0)
		cx := float32(t.Get("clientX").Float())
		cy := float32(t.Get("clientY").Float())
		dragRotY += (cx - dragLastX) * 0.005
		dragRotX += (cy - dragLastY) * 0.005
		dragLastX = cx
		dragLastY = cy
		return nil
	}))
	canvasEl.Call("addEventListener", "touchend", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		dragging = false
		return nil
	}))

	// Event: scroll wheel zoom
	canvasEl.Call("addEventListener", "wheel", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		e.Call("preventDefault")
		delta := float32(e.Get("deltaY").Float()) * 0.1
		zoomVal := float32(js.Global().Get("parseFloat").Invoke(cameraControl.Get("value")).Float())
		zoomVal -= delta
		if zoomVal < -95 {
			zoomVal = -95
		}
		if zoomVal > 95 {
			zoomVal = 95
		}
		cameraControl.Set("value", strconv.FormatFloat(float64(zoomVal), 'f', 0, 64))
		sliderZoom.Set("textContent", strconv.FormatFloat(float64(zoomVal), 'f', 0, 64))
		return nil
	}))

	// Initial mode — read from URL hash if present
	selectedMode = "globe"
	hash := js.Global().Get("location").Get("hash").String()
	if len(hash) > 1 {
		hashMode := hash[1:]
		// Validate it's a known mode
		if _, ok := attractorParams[hashMode]; ok {
			selectedMode = hashMode
		} else {
			// Check non-parameterized modes
			switch hashMode {
			case "tetrahedron", "cube", "octahedron", "dodecahedron", "icosahedron", "nestedcube", "globe", "magnetosphere":
				selectedMode = hashMode
			}
		}
	}
	// Check the matching radio button
	radio := doc.Call("querySelector", `input[name="mode"][value="`+selectedMode+`"]`)
	if !radio.IsNull() && !radio.IsUndefined() {
		radio.Set("checked", true)
	}
	buildParamPanel(selectedMode)

	// Initialize persistent JS typed arrays for zero-alloc frame uploads
	jsVertUint8 = js.Global().Get("Uint8Array").New(steps * 4 * 4)
	buf := jsVertUint8.Get("buffer")
	jsVertFloat = js.Global().Get("Float32Array").New(buf, 0, steps*4)

	// Initialize WebGL
	glTypes.New(gl)
	attractorDrawMode = glTypes.LineStrip
	// Bind buffers before setting up attrib pointers in setupShaders
	gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
	gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
	setupShaders()
	setupMatrices()
	generateForMode(selectedMode)
	autoFitCamera()
	refreshGradient()

	// Check if debug mode is enabled via JS global
	debugVal := js.Global().Get("__WASM_DEBUG__")
	if !debugVal.IsUndefined() && debugVal.Bool() {
		debugEnabled = true
	}

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

	// Debug stats reporter goroutine
	if debugEnabled {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				postDebugStats()
			}
		}()
	}

	<-done
}

func postDebugStats() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	avgMs := float32(0)
	fps := float32(0)
	if frameCount > 0 {
		avgMs = frameTotalMs / float32(frameCount)
		fps = 1000.0 / avgMs
	}

	payload := fmt.Sprintf(
		`{"mode":"%s","paused":%t,"fps":%.1f,"frame_avg_ms":%.2f,"frame_min_ms":%.2f,"frame_max_ms":%.2f,"frame_count":%d,"speed_steps":%d,"speed_scale":%.4f,"trail_steps":%d,"heap_alloc_mb":%.2f,"heap_sys_mb":%.2f,"heap_objects":%d,"gc_runs":%d,"goroutines":%d}`,
		selectedMode, paused, fps, avgMs, frameMinMs, frameMaxMs, frameCount,
		speedSteps, speedScale, steps,
		float64(ms.HeapAlloc)/1048576, float64(ms.HeapSys)/1048576,
		ms.HeapObjects, gcRunsCount(&ms), runtime.NumGoroutine(),
	)

	// Reset frame stats for next interval
	frameCount = 0
	frameTotalMs = 0
	frameMinMs = 999
	frameMaxMs = 0

	// Post via fetch
	headers := js.Global().Get("Headers").New()
	headers.Call("set", "Content-Type", "application/json")
	opts := js.Global().Get("Object").New()
	opts.Set("method", "POST")
	opts.Set("headers", headers)
	opts.Set("body", payload)
	js.Global().Call("fetch", "/debug/stats", opts)
}

// Per-attractor initial conditions — defaults to (0.1, 0.5, -0.6) for most.
var attractorInitCond = map[string][3]float32{
	"chua":       {0.1, 0.0, 0.0},
	"rabinovich": {-1.0, 0.0, 0.5},
	"burkeshaw":  {0.6, 0.0, 0.0},
	"chen":       {-3.0, 2.0, 20.0},
	"sprott":     {0.63, 0.47, -0.54},
	"thomas":     {1.0, 0.0, 0.0},
	"halvorsen":  {-1.48, -1.51, 2.04},
}

var attractorDescriptions = map[string]string{
	"lorenz": "Lorenz Attractor — Discovered by Edward Lorenz in 1963 while modeling atmospheric convection. " +
		"The butterfly-shaped trajectory arises from a simplified system of three coupled differential equations. " +
		"It was one of the first systems shown to exhibit deterministic chaos, where tiny differences in initial conditions lead to vastly different outcomes.\n\n" +
		"dx/dt = σ(y − x)\ndy/dt = x(ρ − z) − y\ndz/dt = xy − βz",
	"rossler": "Rössler Attractor — Proposed by Otto Rössler in 1976 as a simpler system that produces chaotic behavior. " +
		"Unlike the Lorenz system's two-lobed shape, the Rössler attractor has a single folded-band structure with an outward spiral that occasionally makes a large excursion in the z-direction.\n\n" +
		"dx/dt = −(y + z)\ndy/dt = x + ay\ndz/dt = b + z(x − c)",
	"chua": "Chua's Circuit (Double Scroll Attractor) — Invented by Leon Chua in 1983, this is the first electronic circuit proven to exhibit chaos. " +
		"The system features a piecewise-linear nonlinearity (the Chua diode) that creates the characteristic double-scroll pattern. " +
		"It is also the basis for multi-scroll attractor generalizations.\n\n" +
		"dx/dt = α(y − x − h(x))\ndy/dt = x − y + z\ndz/dt = −βy\nh(x) = m₁x + ½(m₀ − m₁)(|x+1| − |x−1|)",
	"aizawa": "Aizawa Attractor — A chaotic system that produces a toroidal structure with a tendril extending from the center. " +
		"The attractor has a visually striking shape that resembles a sphere with a tail, exhibiting both rotational symmetry and chaotic wandering.\n\n" +
		"dx/dt = (z − b)x − dy\ndy/dt = dx + (z − b)y\ndz/dt = c + az − z³/3 − (x² + y²)(1 + ez) + fzx³",
	"sprott": "Sprott Attractor — One of many simple chaotic systems catalogued by Julien Clinton Sprott. " +
		"These systems were discovered through systematic computer searches for chaotic flows with minimal terms, demonstrating that chaos can arise from remarkably simple equations.\n\n" +
		"dx/dt = y + Axy + xz\ndy/dt = 1 − Bx² + yz\ndz/dt = x − x² − y²",
	"lissajou": "Lissajous Curve — Named after Jules Antoine Lissajous (1822–1880), these are parametric curves formed by combining sinusoidal motions along each axis. " +
		"Not a chaotic system — the curves are periodic and their shape depends on the frequency ratios and phase relationships between the three oscillations.\n\n" +
		"x(t) = sin(at)\ny(t) = sin(bt)\nz(t) = sin(ct)",
	"thomas": "Thomas' Cyclically Symmetric Attractor — Introduced by René Thomas, this system has the elegant property of cyclic symmetry: each variable is damped and driven by the sine of the next variable in the cycle. " +
		"The parameter b controls dissipation; as b decreases the system transitions from stable points through limit cycles to chaos.\n\n" +
		"dx/dt = −bx + sin(y)\ndy/dt = −by + sin(z)\ndz/dt = −bz + sin(x)",
	"halvorsen": "Halvorsen Attractor — A chaotic system with three-fold rotational symmetry, producing a distinctive pinwheel-like shape. " +
		"The attractor consists of three intertwined lobes that spiral around each other, creating a visually complex but structurally symmetric trajectory.\n\n" +
		"dx/dt = −ax − 4y − 4z − y²\ndy/dt = −ay − 4z − 4x − z²\ndz/dt = −az − 4x − 4y − x²",
	"chen": "Chen Attractor — Discovered by Guanrong Chen in 1999, this system was found as a dual of the Lorenz system in a specific mathematical sense. " +
		"It exhibits chaotic behavior with a distinctive two-scroll structure that differs from both the Lorenz and Rössler attractors.\n\n" +
		"dx/dt = a(y − x)\ndy/dt = (c − a)x − xz + cy\ndz/dt = xy − bz",
	"dadras": "Dadras Attractor — A three-dimensional autonomous chaotic system with five parameters, introduced by Sara Dadras and Hamid Reza Momeni. " +
		"The system exhibits rich dynamical behavior including period-doubling routes to chaos.\n\n" +
		"dx/dt = y − px + qyz\ndy/dt = ry − xz + z\ndz/dt = sxy − ez",
	"rabinovich": "Rabinovich-Fabrikant Attractor — Derived by Mikhail Rabinovich and Anatoly Fabrikant from physical equations modeling the stochasticity of three interacting waves. " +
		"The system is known for its complex topology and extreme sensitivity to parameters, producing intricate folded structures.\n\n" +
		"dx/dt = y(z − 1 + x²) + γx\ndy/dt = x(3z + 1 − x²) + γy\ndz/dt = −2z(α + xy)",
	"burkeshaw": "Burke-Shaw Attractor — Introduced by Bill Burke and Robert Shaw, this system exhibits chaotic behavior with a distinctive two-winged structure. " +
		"It arises from the study of nonlinear dynamics and produces complex trajectories confined to a compact region of phase space.\n\n" +
		"dx/dt = −S(x + y)\ndy/dt = −y − Sxz\ndz/dt = Sxy + V",
	"tetrahedron":    "Tetrahedron — The simplest Platonic solid, with 4 triangular faces, 6 edges, and 4 vertices. It is its own dual.",
	"cube":           "Cube (Hexahedron) — A Platonic solid with 6 square faces, 12 edges, and 8 vertices. Its dual is the octahedron.",
	"octahedron":     "Octahedron — A Platonic solid with 8 triangular faces, 12 edges, and 6 vertices. Its dual is the cube.",
	"dodecahedron":   "Dodecahedron — A Platonic solid with 12 pentagonal faces, 30 edges, and 20 vertices. Its dual is the icosahedron.",
	"icosahedron":    "Icosahedron — A Platonic solid with 20 triangular faces, 30 edges, and 12 vertices. Its dual is the dodecahedron.",
	"nestedcube":     "Nested Cube — A cube within a cube, connected at the vertices, illustrating the relationship between inner and outer geometric structures.",
	"globe":          "Globe — A wireframe sphere showing lines of latitude and longitude, similar to the graticule on a geographic globe. Latitude lines are horizontal circles parallel to the equator, longitude lines are great circles passing through the poles.",
	"sphere":         "Sphere — A perfectly round three-dimensional surface where every point is equidistant from the center. Generated as a UV sphere with configurable latitude and longitude subdivisions.",
	"torus":          "Torus — A doughnut-shaped surface of revolution generated by revolving a circle (radius r) around an axis at distance R from the center of the circle.",
	"magnetosphere":  "Magnetosphere — A visualization of magnetic field lines surrounding a dipole, similar to Earth's magnetosphere that shields the planet from solar wind.",
}

func resetAttractorState() {
	if ic, ok := attractorInitCond[selectedMode]; ok {
		x, y, z = ic[0], ic[1], ic[2]
	} else {
		x, y, z = 0.1, 0.5, -0.6
	}
	centerReady = false
	centerWarmup = 0
}

// checkDiverged returns true and resets state if the attractor has diverged (NaN or >1e6).
func checkDiverged() bool {
	if x != x || y != y || z != z || x > 1e6 || x < -1e6 || y > 1e6 || y < -1e6 || z > 1e6 || z < -1e6 {
		resetAttractorState()
		return true
	}
	return false
}

// applySpeedLog converts a log10 slider value into speedSteps and speedScale.
// Slider range -2..2 maps to effective speed 0.01..100.
// Values >= 1: sub-step (speedSteps=N, speedScale=1.0).
// Values < 1: scale dt down (speedSteps=1, speedScale=fraction).
func applySpeedLog(logVal float64) {
	speed := math.Pow(10, logVal)
	if speed >= 1.0 {
		speedSteps = int(speed + 0.5)
		speedScale = 1.0
	} else {
		speedSteps = 1
		speedScale = float32(speed)
	}
	// Display the effective speed value
	var label string
	if speed >= 1.0 {
		label = strconv.Itoa(speedSteps)
	} else {
		label = strconv.FormatFloat(speed, 'f', 2, 64)
	}
	doc.Call("getElementById", "slider-value-speed").Set("textContent", label)
}

// ── UI helpers ───────────────────────────────────────────────────────────────

// decimalsForStep returns the number of decimal places needed to represent a step value.
func decimalsForStep(step float32) int {
	s := strconv.FormatFloat(float64(step), 'f', 10, 32)
	dot := -1
	for i, c := range s {
		if c == '.' {
			dot = i
			break
		}
	}
	if dot < 0 {
		return 0
	}
	// Find last non-zero digit after the dot
	last := dot
	for i := len(s) - 1; i > dot; i-- {
		if s[i] != '0' {
			last = i
			break
		}
	}
	return last - dot
}

func buildParamPanel(mode string) {
	paramsDiv := doc.Call("getElementById", "params")
	paramsDiv.Set("innerHTML", "")

	params, ok := attractorParams[mode]
	if !ok || len(params) == 0 {
		return
	}

	for _, p := range params {
		p := p // capture for closure
		dec := decimalsForStep(p.Step)

		span := doc.Call("createElement", "span")
		span.Set("style", "margin-right:10px;color:#888;display:inline-block;")

		lbl := doc.Call("createElement", "span")
		lbl.Set("textContent", p.Label+" ")
		span.Call("appendChild", lbl)

		stepStr := strconv.FormatFloat(float64(p.Step), 'g', -1, 32)
		minStr := strconv.FormatFloat(float64(p.Min), 'g', -1, 32)
		maxStr := strconv.FormatFloat(float64(p.Max), 'g', -1, 32)

		slider := doc.Call("createElement", "input")
		slider.Set("type", "range")
		slider.Set("id", p.ID)
		slider.Set("min", minStr)
		slider.Set("max", maxStr)
		slider.Set("value", strconv.FormatFloat(float64(*p.Value), 'g', -1, 32))
		slider.Set("step", stepStr)
		slider.Set("style", "width:80px;vertical-align:middle;")

		numInput := doc.Call("createElement", "input")
		numInput.Set("type", "number")
		numInput.Set("min", minStr)
		numInput.Set("max", maxStr)
		numInput.Set("step", stepStr)
		numInput.Set("value", strconv.FormatFloat(float64(*p.Value), 'f', dec, 64))
		numInput.Set("style", "width:60px;background:#222;color:#ccc;border:1px solid #555;font-family:monospace;font-size:11px;margin-left:2px;vertical-align:middle;")

		// Slider → number input
		slider.Call("addEventListener", "input", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			val, err := strconv.ParseFloat(slider.Get("value").String(), 64)
			if err == nil {
				*p.Value = float32(val)
				numInput.Set("value", strconv.FormatFloat(val, 'f', dec, 64))
				resetAttractorState()
				refreshGradient()
			}
			return nil
		}))

		// Number input → slider
		numInput.Call("addEventListener", "input", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			val, err := strconv.ParseFloat(numInput.Get("value").String(), 64)
			if err == nil {
				*p.Value = float32(val)
				slider.Set("value", strconv.FormatFloat(val, 'g', -1, 64))
				resetAttractorState()
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
			slider.Set("value", defStr)
			numInput.Set("value", strconv.FormatFloat(float64(p.Def), 'f', dec, 64))
			resetAttractorState()
			refreshGradient()
			return nil
		}))

		// Step-size input
		stepInput := doc.Call("createElement", "input")
		stepInput.Set("type", "number")
		stepInput.Set("min", "0.0000001")
		stepInput.Set("step", "any")
		stepInput.Set("value", stepStr)
		stepInput.Set("title", "Step size")
		stepInput.Set("style", "width:50px;background:#1a1a1a;color:#666;border:1px solid #444;font-family:monospace;font-size:10px;margin-left:1px;vertical-align:middle;")
		stepInput.Call("addEventListener", "input", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			val, err := strconv.ParseFloat(stepInput.Get("value").String(), 64)
			if err == nil && val > 0 {
				newStep := strconv.FormatFloat(val, 'g', -1, 64)
				slider.Set("step", newStep)
				numInput.Set("step", newStep)
			}
			return nil
		}))

		span.Call("appendChild", slider)
		span.Call("appendChild", numInput)
		span.Call("appendChild", rst)
		span.Call("appendChild", stepInput)
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

func updateInfoOverlay() {
	overlay := doc.Call("getElementById", "info-overlay")
	if overlay.IsNull() || overlay.IsUndefined() {
		return
	}
	showInfo := doc.Call("getElementById", "show-info")
	if showInfo.IsNull() || showInfo.IsUndefined() || !showInfo.Get("checked").Bool() {
		return
	}
	if desc, ok := attractorDescriptions[selectedMode]; ok {
		overlay.Set("textContent", desc)
	} else {
		overlay.Set("textContent", selectedMode)
	}
}

func onModeChange(this js.Value, args []js.Value) interface{} {
	selectedMode = doc.Call("querySelector", `input[name="mode"]:checked`).Get("value").String()
	resetAttractorState()
	buildParamPanel(selectedMode)
	updateInfoOverlay()
	// Update URL hash
	js.Global().Get("location").Set("hash", selectedMode)
	// Run one frame to populate vertices, then update gradient and fit camera
	generateForMode(selectedMode)
	autoFitCamera()
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
	rotationX, rotationY, rotationZ = 0, 0, 0
	rotationX1, rotationY1, rotationZ1 = 0, 0, 0
	movMatrix = mgl32.Ident4()

	// Reset attractor position
	resetAttractorState()

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

	// Reset speed, auto-rotate, draw mode, trail
	paused = false
	doc.Call("getElementById", "pause-btn").Set("textContent", "Pause")
	speedSteps = 1
	speedScale = 1.0
	autoRotate = true
	usePoints = false
	attractorDrawMode = glTypes.LineStrip
	dragRotX, dragRotY = 0, 0
	doc.Call("getElementById", "speed-slider").Set("value", "0")
	doc.Call("getElementById", "slider-value-speed").Set("textContent", "1")
	doc.Call("getElementById", "auto-rotate").Set("checked", true)
	doc.Call("getElementById", "use-points").Set("checked", false)
	doc.Call("getElementById", "show-info").Set("checked", false)
	doc.Call("getElementById", "info-overlay").Get("style").Set("display", "none")
	if steps != 20000 {
		steps = 20000
		vertBuf = make([]float32, steps*4)
		jsVertUint8 = js.Global().Get("Uint8Array").New(steps * 4 * 4)
		buf := jsVertUint8.Get("buffer")
		jsVertFloat = js.Global().Get("Float32Array").New(buf, 0, steps*4)
	}
	doc.Call("getElementById", "trail-slider").Set("value", "20000")
	doc.Call("getElementById", "slider-value-trail").Set("textContent", "20000")
	persistTrail = false
	doc.Call("getElementById", "persist-trail").Set("checked", false)
	gradientMode = 0
	gradientReverse = false
	doc.Call("getElementById", "gradient-type").Set("value", "z2")
	doc.Call("getElementById", "gradient-reverse").Set("checked", false)

	// Reset colors
	baseColor = [3]float32{1.0, 0.0, 0.0}
	midColor = [3]float32{0.0, 1.0, 0.0}
	topColor = [3]float32{0.0, 0.0, 1.0}
	bgColor = [3]float32{0, 0, 0}
	doc.Call("getElementById", "color-base").Set("value", "#ff0000")
	doc.Call("getElementById", "color-mid").Set("value", "#00ff00")
	doc.Call("getElementById", "color-top").Set("value", "#0000ff")
	doc.Call("getElementById", "color-bg").Set("value", "#000000")
	gl.Call("uniform3f", uBaseColorLoc, baseColor[0], baseColor[1], baseColor[2])
	gl.Call("uniform3f", uMidColorLoc, midColor[0], midColor[1], midColor[2])
	gl.Call("uniform3f", uTopColorLoc, topColor[0], topColor[1], topColor[2])
	gl.Call("clearColor", 0, 0, 0, 1.0)

	// Reset view
	generateForMode(selectedMode)
	updateViewMatrix()
	updateModelMatrix()

	return nil
}
