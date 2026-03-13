//go:build js && wasm

package main

var chenDT, chenA, chenB, chenC float32 = 0.002, 5.0, -10.0, -0.38

func generateChen() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := chenDT * speedMult
		x1 := x + dt*(chenA*x-y*z)
		y1 := y + dt*(chenB*y+x*z)
		z1 := z + dt*(chenC*z+x*y/3)
		x, y, z = x1, y1, z1
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
