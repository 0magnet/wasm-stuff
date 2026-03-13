//go:build js && wasm

package main

import "math"

var (
	sphereRadius   float32 = 1.0
	sphereStacks   int     = 30
	sphereSlices   int     = 30
	torusR         float32 = 1.5
	torusr         float32 = 0.5
	torusStacks    int     = 30
	torusSlices    int     = 30
)

func generateSphere() {
	var vertices []float32
	var indices []uint16
	for i := 0; i <= sphereStacks; i++ {
		phi := float32(i) * float32(math.Pi) / float32(sphereStacks)
		for j := 0; j <= sphereSlices; j++ {
			theta := float32(j) * 2.0 * float32(math.Pi) / float32(sphereSlices)
			xv := sphereRadius * float32(math.Sin(float64(phi))) * float32(math.Cos(float64(theta)))
			yv := sphereRadius * float32(math.Sin(float64(phi))) * float32(math.Sin(float64(theta)))
			zv := sphereRadius * float32(math.Cos(float64(phi)))
			vertices = append(vertices, xv, yv, zv)
		}
	}
	for i := 0; i < sphereStacks; i++ {
		for j := 0; j <= sphereSlices; j++ {
			indices = append(indices, uint16(i*(sphereSlices+1)+j), uint16((i+1)*(sphereSlices+1)+j))
		}
	}
	attractorVertices = vertices
	attractorIndices = indices
	gl.Call("bindBuffer", glTypes.ArrayBuffer, attractorVertexBuffer)
	gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(attractorVertices), glTypes.StaticDraw)
	gl.Call("bindBuffer", glTypes.ElementArrayBuffer, attractorIndexBuffer)
	gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(attractorIndices), glTypes.StaticDraw)
	gl.Call("drawElements", glTypes.Line, len(attractorIndices), glTypes.UnsignedShort, 0)
	gl.Call("drawArrays", glTypes.LineLoop, 0, len(attractorVertices)/3)
}

func generateTorus() {
	var vertices []float32
	var indices []uint16
	for i := 0; i < torusStacks; i++ {
		theta := float32(i) * 2.0 * math.Pi / float32(torusStacks)
		for j := 0; j <= torusSlices; j++ {
			phi := float32(j) * 2.0 * math.Pi / float32(torusSlices)
			xv := (torusR + torusr*float32(math.Cos(float64(phi)))) * float32(math.Cos(float64(theta)))
			yv := (torusR + torusr*float32(math.Cos(float64(phi)))) * float32(math.Sin(float64(theta)))
			zv := torusr * float32(math.Sin(float64(phi)))
			vertices = append(vertices, xv, yv, zv)
		}
	}
	for j := 0; j < torusStacks; j++ {
		for i := 0; i < torusSlices; i++ {
			first := uint16((j * (torusSlices + 1)) + i)
			second := first + 1
			third := first + uint16(torusSlices) + 1
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
	gl.Call("drawArrays", glTypes.LineLoop, 0, len(attractorVertices)/3)
}

func generateMagnetosphere() {
	generateSphere()
	generateTorus()
	// second outer torus
	savedR := torusR
	torusR = 2.0
	generateTorus()
	torusR = savedR
}
