//go:build js && wasm

package main

var rosslerDT, rosslerA, rosslerB, rosslerC float32 = 0.005, 0.2, 0.2, 5.7

func generateRossler() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		x1 := x + rosslerDT*(-y-z)
		y1 := y + rosslerDT*(x+rosslerA*y)
		z1 := z + rosslerDT*(rosslerB+z*(x-rosslerC))
		x, y, z = x1, y1, z1
		vertices[i*3] = x
		vertices[i*3+1] = y
		vertices[i*3+2] = z
	}
	uploadVerticesOnly(vertices, glTypes.LineStrip, len(vertices)/3)
}
