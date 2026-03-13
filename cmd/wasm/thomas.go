//go:build js && wasm

package main

import "math"

var thomasDT, thomasB float32 = 0.05, 0.208186

func generateThomas() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		dt := thomasDT * speedMult
		x1 := x + dt*(-thomasB*x+float32(math.Sin(float64(y))))
		y1 := y + dt*(-thomasB*y+float32(math.Sin(float64(z))))
		z1 := z + dt*(-thomasB*z+float32(math.Sin(float64(x))))
		x, y, z = x1, y1, z1
		vertices[i*3], vertices[i*3+1], vertices[i*3+2] = x, y, z
	}
	uploadVerticesOnly(vertices, attractorDrawMode, len(vertices)/3)
}
