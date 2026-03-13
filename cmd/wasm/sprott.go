//go:build js && wasm

package main

var sprottDT, sprottA, sprottB float32 = 0.01, 2.07, 1.8

func generateSprott() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := sprottDT * speedMult
		x1 := x + dt*(y+sprottA*x*y+x*z)
		y1 := y + dt*(1-sprottB*x*x+y*z)
		z1 := z + dt*(x-x*x-y*y)
		x, y, z = x1, y1, z1
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
