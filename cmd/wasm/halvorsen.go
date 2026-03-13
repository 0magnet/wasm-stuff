//go:build js && wasm

package main

var halvorsenDT, halvorsenA float32 = 0.005, 1.89

func generateHalvorsen() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		dt := halvorsenDT * speedMult
		x1 := x + dt*(-halvorsenA*x-4*y-4*z-y*y)
		y1 := y + dt*(-halvorsenA*y-4*z-4*x-z*z)
		z1 := z + dt*(-halvorsenA*z-4*x-4*y-x*x)
		x, y, z = x1, y1, z1
		vertices[i*3], vertices[i*3+1], vertices[i*3+2] = x, y, z
	}
	uploadVerticesOnly(vertices, attractorDrawMode, len(vertices)/3)
}
