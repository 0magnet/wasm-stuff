//go:build js && wasm

package attractor

import (
	"syscall/js"

	sg "github.com/0magnet/audioprism-go/pkg/spectrogram"
)

// The spectrogram mode paints a scrolling 2D texture on the shared
// #gocanvas: newest FFT column at the right edge, older columns slide
// left, oldest wraps around. It owns its own shader program (a plain
// textured quad with a scroll uniform) so it doesn't fight the 3D
// attractor pipeline; the mode switch handles use-program bookkeeping.
//
// Sample flow: audiosrc.Source → snapshot per rAF frame → FFT → color
// column → texSubImage2D one column at (texCol % width). uOffset in the
// fragment shader scrolls the whole texture so the newest column always
// lands at the right edge.

const spectVertShaderSrc = `
	attribute vec2 aPos;
	attribute vec2 aTex;
	varying vec2 vTex;
	void main() {
		gl_Position = vec4(aPos, 0.0, 1.0);
		vTex = aTex;
	}
`

const spectFragShaderSrc = `
	precision mediump float;
	varying vec2 vTex;
	uniform sampler2D uSampler;
	uniform float uOffset;
	void main(void) {
		float x = mod(vTex.x + uOffset, 1.0);
		gl_FragColor = texture2D(uSampler, vec2(x, vTex.y));
	}
`

var (
	spectProgram    js.Value
	spectTexture    js.Value
	spectVBuf       js.Value
	spectTBuf       js.Value
	spectAPos       js.Value
	spectATex       js.Value
	spectUSampler   js.Value
	spectUOffset    js.Value
	spectReady      bool
	spectTexCol     int
	spectTexW       int
	spectTexH       int
	spectColUint8   js.Value // reused Uint8Array of length spectTexH*4
	spectSampleBuf  []float32
	spectMagnitudes []float64
)

func initSpectrogram() {
	if spectReady {
		return
	}
	// Own shader program.
	vs := gl.Call("createShader", glTypes.VertexShader)
	gl.Call("shaderSource", vs, spectVertShaderSrc)
	gl.Call("compileShader", vs)
	fs := gl.Call("createShader", glTypes.FragmentShader)
	gl.Call("shaderSource", fs, spectFragShaderSrc)
	gl.Call("compileShader", fs)

	spectProgram = gl.Call("createProgram")
	gl.Call("attachShader", spectProgram, vs)
	gl.Call("attachShader", spectProgram, fs)
	gl.Call("linkProgram", spectProgram)

	spectAPos = gl.Call("getAttribLocation", spectProgram, "aPos")
	spectATex = gl.Call("getAttribLocation", spectProgram, "aTex")
	spectUSampler = gl.Call("getUniformLocation", spectProgram, "uSampler")
	spectUOffset = gl.Call("getUniformLocation", spectProgram, "uOffset")

	// Fullscreen quad geometry — two triangles as a TRIANGLE_STRIP.
	quadPos := []float32{-1, -1, 1, -1, -1, 1, 1, 1}
	quadTex := []float32{0, 0, 1, 0, 0, 1, 1, 1}
	spectVBuf = gl.Call("createBuffer")
	gl.Call("bindBuffer", glTypes.ArrayBuffer, spectVBuf)
	gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(quadPos), glTypes.StaticDraw)
	spectTBuf = gl.Call("createBuffer")
	gl.Call("bindBuffer", glTypes.ArrayBuffer, spectTBuf)
	gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(quadTex), glTypes.StaticDraw)

	// Texture sized to the current canvas dimensions. Width = number of
	// columns (time axis); height = FFT bins (frequency axis).
	spectTexW = width
	spectTexH = height
	spectTexture = gl.Call("createTexture")
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), spectTexture)
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_MIN_FILTER"), gl.Get("LINEAR"))
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_MAG_FILTER"), gl.Get("LINEAR"))
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_WRAP_S"), gl.Get("CLAMP_TO_EDGE"))
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_WRAP_T"), gl.Get("CLAMP_TO_EDGE"))
	zeroAB := js.Global().Get("ArrayBuffer").New(spectTexW * spectTexH * 4)
	zeroU8 := js.Global().Get("Uint8Array").New(zeroAB)
	gl.Call("texImage2D",
		gl.Get("TEXTURE_2D"), 0, gl.Get("RGBA"),
		spectTexW, spectTexH, 0,
		gl.Get("RGBA"), gl.Get("UNSIGNED_BYTE"), zeroU8)

	colAB := js.Global().Get("ArrayBuffer").New(spectTexH * 4)
	spectColUint8 = js.Global().Get("Uint8Array").New(colAB)

	spectSampleBuf = make([]float32, sg.FFTSize)
	spectReady = true
}

func teardownSpectrogram() {
	if !spectReady {
		return
	}
	if !spectTexture.IsUndefined() {
		gl.Call("deleteTexture", spectTexture)
	}
	if !spectVBuf.IsUndefined() {
		gl.Call("deleteBuffer", spectVBuf)
	}
	if !spectTBuf.IsUndefined() {
		gl.Call("deleteBuffer", spectTBuf)
	}
	if !spectProgram.IsUndefined() {
		gl.Call("deleteProgram", spectProgram)
	}
	spectTexture = js.Undefined()
	spectVBuf = js.Undefined()
	spectTBuf = js.Undefined()
	spectProgram = js.Undefined()
	spectReady = false
	spectTexCol = 0
	spectSampleBuf = nil
}

func renderSpectrogramFrame() {
	if !spectReady {
		initSpectrogram()
	}
	src := ensureAudioSource()

	// If the canvas has resized since init, throw away our texture and
	// rebuild. Cheap because the texture is the only sized resource.
	if spectTexW != width || spectTexH != height {
		teardownSpectrogram()
		initSpectrogram()
	}

	// Pull the most recent window from the mic and paint one column.
	// This runs at rAF rate (~60Hz), not the natural spectrogram column
	// rate (SampleRate/StepSize ≈ 23Hz at 24kHz). That gives slightly
	// more visual detail per second than a strictly time-locked scroll,
	// which is fine for a plumbing pass — a follow-up can add sample
	// locking à la audioprism-go's queue.
	if src != nil && src.Ready() {
		src.TimeDomain(spectSampleBuf)
		spectMagnitudes = sg.ComputeFFT(spectSampleBuf)
		writeSpectColumn(spectMagnitudes)
	}

	gl.Call("useProgram", spectProgram)
	gl.Call("disable", glTypes.DepthTest)
	gl.Call("clearColor", 0, 0, 0, 1)
	gl.Call("clear", glTypes.ColorBufferBit)

	gl.Call("bindBuffer", glTypes.ArrayBuffer, spectVBuf)
	gl.Call("enableVertexAttribArray", spectAPos)
	gl.Call("vertexAttribPointer", spectAPos, 2, glTypes.Float, false, 0, 0)
	gl.Call("bindBuffer", glTypes.ArrayBuffer, spectTBuf)
	gl.Call("enableVertexAttribArray", spectATex)
	gl.Call("vertexAttribPointer", spectATex, 2, glTypes.Float, false, 0, 0)

	gl.Call("activeTexture", gl.Get("TEXTURE0"))
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), spectTexture)
	gl.Call("uniform1i", spectUSampler, 0)
	// uOffset shifts so the newest column always lands at the right edge.
	gl.Call("uniform1f", spectUOffset, float64(spectTexCol)/float64(spectTexW))
	gl.Call("drawArrays", gl.Get("TRIANGLE_STRIP"), 0, 4)
}

func writeSpectColumn(mags []float64) {
	if len(mags) == 0 || spectTexH == 0 {
		return
	}
	col := make([]byte, spectTexH*4)
	sampleRate := 24000
	if audioSource != nil && audioSource.SampleRate() > 0 {
		sampleRate = audioSource.SampleRate()
	}
	// Display band: 0 to sampleRate/4 (up to Nyquist/2 so low freqs are
	// legible). Callers wanting the full band can adjust this later.
	maxFreq := float64(sampleRate) / 4.0
	for y := 0; y < spectTexH; y++ {
		// Flip Y so low freqs are at the bottom (image y=0 is top).
		yFlipped := spectTexH - 1 - y
		freq := float64(yFlipped) / float64(spectTexH) * maxFreq
		bin := int(freq * float64(sg.FFTSize) / float64(sampleRate))
		if bin < 0 || bin >= len(mags) {
			continue
		}
		c := sg.MagnitudeToPixel(mags[bin])
		r, g, b, a := c.RGBA()
		col[y*4+0] = byte(r >> 8)
		col[y*4+1] = byte(g >> 8)
		col[y*4+2] = byte(b >> 8)
		col[y*4+3] = byte(a >> 8)
	}
	js.CopyBytesToJS(spectColUint8, col)
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), spectTexture)
	gl.Call("texSubImage2D",
		gl.Get("TEXTURE_2D"), 0,
		spectTexCol, 0, 1, spectTexH,
		gl.Get("RGBA"), gl.Get("UNSIGNED_BYTE"), spectColUint8)
	spectTexCol = (spectTexCol + 1) % spectTexW
}
