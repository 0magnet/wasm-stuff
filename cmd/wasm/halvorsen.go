//go:build js && wasm

package main

var halvorsenDT, halvorsenA float32 = 0.005, 1.89

func generateHalvorsen() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := halvorsenDT * speedMult
		x1 := x + dt*(-halvorsenA*x-4*y-4*z-y*y)
		y1 := y + dt*(-halvorsenA*y-4*z-4*x-z*z)
		z1 := z + dt*(-halvorsenA*z-4*x-4*y-x*x)
		x, y, z = x1, y1, z1
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
