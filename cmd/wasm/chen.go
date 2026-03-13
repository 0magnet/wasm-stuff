//go:build js && wasm

package main

var chenDT, chenA, chenB, chenC float32 = 0.002, 5.0, -10.0, -0.38

func generateChen() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		dt := chenDT * speedMult
		x1 := x + dt*(chenA*x-y*z)
		y1 := y + dt*(chenB*y+x*z)
		z1 := z + dt*(chenC*z+x*y/3)
		x, y, z = x1, y1, z1
		vertices[i*3], vertices[i*3+1], vertices[i*3+2] = x, y, z
	}
	uploadVerticesOnly(vertices, attractorDrawMode, len(vertices)/3)
}
