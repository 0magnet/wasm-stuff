//go:build js && wasm

package main

import "math"

var thomasDT, thomasB float32 = 0.05, 0.208186

func generateThomas() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := thomasDT * speedScale
		for s := 0; s < speedSteps; s++ {
			x1 := x + dt*(-thomasB*x+float32(math.Sin(float64(y))))
			y1 := y + dt*(-thomasB*y+float32(math.Sin(float64(z))))
			z1 := z + dt*(-thomasB*z+float32(math.Sin(float64(x))))
			x, y, z = x1, y1, z1
			checkDiverged()
		}
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
