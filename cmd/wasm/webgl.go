//go:build js && wasm

package main

import (
	"math"
	"runtime"
	"syscall/js"
	"unsafe"
)

// GLTypes holds WebGL constant values.
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
	Points             js.Value
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
	types.Points = gl.Get("POINTS")
	types.DynamicDraw = gl.Get("DYNAMIC_DRAW")
}

// updateGradientRange scans vertices (stride 3) and sets min/max uniforms for x and z.
// Only called on mode/param change, NOT per frame.
func updateGradientRange(vertices []float32) {
	if !shadersReady || len(vertices) < 3 {
		return
	}
	minX := float32(math.MaxFloat32)
	maxX := float32(-math.MaxFloat32)
	minZ := float32(math.MaxFloat32)
	maxZ := float32(-math.MaxFloat32)
	for i := 0; i < len(vertices); i += 3 {
		if vertices[i] < minX {
			minX = vertices[i]
		}
		if vertices[i] > maxX {
			maxX = vertices[i]
		}
		if vertices[i+2] < minZ {
			minZ = vertices[i+2]
		}
		if vertices[i+2] > maxZ {
			maxZ = vertices[i+2]
		}
	}
	gl.Call("uniform1f", uMinXLoc, float64(minX))
	gl.Call("uniform1f", uMaxXLoc, float64(maxX))
	gl.Call("uniform1f", uMinZLoc, float64(minZ))
	gl.Call("uniform1f", uMaxZLoc, float64(maxZ))
}

// uploadVerticesOnly uploads vertex data and draws with drawArrays (no index buffer).
// Uses persistent JS typed arrays for zero per-frame JS allocation.
func uploadVerticesOnly(vertices []float32, drawMode js.Value, count int) {
	attractorVertices = vertices
	js.CopyBytesToJS(jsVertUint8, sliceToByteSlice(vertices))
	runtime.KeepAlive(vertices)
	gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
	gl.Call("bufferData", glTypes.ArrayBuffer, jsVertFloat, glTypes.StaticDraw)
	gl.Call("drawArrays", drawMode, 0, count)
}

// uploadBuffersIndexed uploads and draws with drawElements.
func uploadBuffersIndexed(vertices []float32, indices []uint16, drawMode js.Value) {
	attractorVertices = vertices
	attractorIndices = indices
	gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
	gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(attractorVertices), glTypes.StaticDraw)
	gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
	gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(attractorIndices), glTypes.StaticDraw)
	gl.Call("drawElements", drawMode, len(attractorIndices), glTypes.UnsignedShort, 0)
}

func sliceToByteSlice(s interface{}) []byte {
	switch s := s.(type) {
	case []int8:
		return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), len(s))
	case []int16:
		return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), len(s)*2)
	case []int32:
		return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), len(s)*4)
	case []int64:
		return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), len(s)*8)
	case []uint8:
		return s
	case []uint16:
		return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), len(s)*2)
	case []uint32:
		return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), len(s)*4)
	case []uint64:
		return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), len(s)*8)
	case []float32:
		return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), len(s)*4)
	case []float64:
		return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), len(s)*8)
	default:
		panic("unexpected value at sliceToByteSlice")
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
		panic("unexpected value at SliceToTypedArray")
	}
}
