//go:build js && wasm

package main

var rosslerDT, rosslerA, rosslerB, rosslerC float32 = 0.005, 0.2, 0.2, 5.7

func generateRossler() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		dt := rosslerDT * speedMult
		x1 := x + dt*(-y-z)
		y1 := y + dt*(x+rosslerA*y)
		z1 := z + dt*(rosslerB+z*(x-rosslerC))
		x, y, z = x1, y1, z1
		vertices[i*3], vertices[i*3+1], vertices[i*3+2] = x, y, z
	}
	uploadVerticesOnly(vertices, attractorDrawMode, len(vertices)/3)
}
