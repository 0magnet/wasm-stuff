//go:build js && wasm

package attractor

import (
	"syscall/js"
)

// The xy scope mode draws (L[i], R[i]) as a line strip on the shared
// #gocanvas — classic oscilloscope-style Lissajous. Mono sources use a
// lag on the same channel to still produce a useful figure (pure
// diagonal on a raw mono signal is not useful; a lagged copy adds an
// orbit).
//
// Own shader program (pass-through vertex + solid-color fragment) with a
// dedicated dynamic-vertex buffer sized to the sample window.

const xyVertShaderSrc = `
	attribute vec2 aPos;
	void main() {
		gl_Position = vec4(aPos, 0.0, 1.0);
	}
`

const xyFragShaderSrc = `
	precision mediump float;
	uniform vec3 uColor;
	uniform float uAlpha;
	void main(void) {
		gl_FragColor = vec4(uColor, uAlpha);
	}
`

var (
	xyProgram   js.Value
	xyBuf       js.Value
	xyAPos      js.Value
	xyUColor    js.Value
	xyUAlpha    js.Value
	xyReady     bool
	xyWindow    = 2048
	xyBufL      []float32
	xyBufR      []float32
	xyLine      []float32 // interleaved x,y pairs for GL upload
	xyScale     float32 = 0.9
	xyMonoLag   int     = 128
)

func initXY() {
	if xyReady {
		return
	}
	vs := gl.Call("createShader", glTypes.VertexShader)
	gl.Call("shaderSource", vs, xyVertShaderSrc)
	gl.Call("compileShader", vs)
	fs := gl.Call("createShader", glTypes.FragmentShader)
	gl.Call("shaderSource", fs, xyFragShaderSrc)
	gl.Call("compileShader", fs)

	xyProgram = gl.Call("createProgram")
	gl.Call("attachShader", xyProgram, vs)
	gl.Call("attachShader", xyProgram, fs)
	gl.Call("linkProgram", xyProgram)

	xyAPos = gl.Call("getAttribLocation", xyProgram, "aPos")
	xyUColor = gl.Call("getUniformLocation", xyProgram, "uColor")
	xyUAlpha = gl.Call("getUniformLocation", xyProgram, "uAlpha")

	xyBuf = gl.Call("createBuffer")
	xyBufL = make([]float32, xyWindow)
	xyBufR = make([]float32, xyWindow)
	xyLine = make([]float32, xyWindow*2)
	xyReady = true
}

func teardownXY() {
	if !xyReady {
		return
	}
	if !xyBuf.IsUndefined() {
		gl.Call("deleteBuffer", xyBuf)
	}
	if !xyProgram.IsUndefined() {
		gl.Call("deleteProgram", xyProgram)
	}
	xyBuf = js.Undefined()
	xyProgram = js.Undefined()
	xyReady = false
}

func renderXYFrame() {
	if !xyReady {
		initXY()
	}
	src := ensureMicSource()

	if src != nil && src.Ready() {
		src.TimeDomainStereo(xyBufL, xyBufR)
		// If the source only has one channel, fall back to a lagged
		// pseudo-stereo so the trace isn't a straight diagonal.
		if src.Channels() < 2 {
			for i := 0; i < xyWindow; i++ {
				j := i - xyMonoLag
				if j < 0 {
					xyBufR[i] = 0
				} else {
					xyBufR[i] = xyBufL[j]
				}
			}
		}
		for i := 0; i < xyWindow; i++ {
			xyLine[i*2+0] = xyBufL[i] * xyScale
			xyLine[i*2+1] = xyBufR[i] * xyScale
		}
	} else {
		// Blank the line so we don't draw stale data.
		for i := range xyLine {
			xyLine[i] = 0
		}
	}

	// Fade previous frame by drawing a translucent black quad-ish clear.
	// WebGL doesn't do a partial-alpha clear, so we just clearColor with
	// alpha < 1 doesn't fade — the browser resets alpha=1. For a proper
	// scope trail effect, the next agent can switch to a persist-color
	// framebuffer. For plumbing, do a full clear each frame.
	gl.Call("disable", glTypes.DepthTest)
	gl.Call("clearColor", 0, 0, 0, 1)
	gl.Call("clear", glTypes.ColorBufferBit)

	gl.Call("useProgram", xyProgram)
	gl.Call("bindBuffer", glTypes.ArrayBuffer, xyBuf)
	gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(xyLine), glTypes.DynamicDraw)
	gl.Call("enableVertexAttribArray", xyAPos)
	gl.Call("vertexAttribPointer", xyAPos, 2, glTypes.Float, false, 0, 0)
	gl.Call("uniform3f", xyUColor, 0.4, 1.0, 0.4)
	gl.Call("uniform1f", xyUAlpha, 1.0)
	gl.Call("drawArrays", glTypes.LineStrip, 0, xyWindow)
}
