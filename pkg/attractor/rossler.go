//go:build js && wasm

package main

var rosslerDT, rosslerA, rosslerB, rosslerC float32 = 0.005, 0.2, 0.2, 5.7

func generateRossler() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := rosslerDT * speedScale
		for s := 0; s < speedSteps; s++ {
			x1 := x + dt*(-y-z)
			y1 := y + dt*(x+rosslerA*y)
			z1 := z + dt*(rosslerB+z*(x-rosslerC))
			x, y, z = x1, y1, z1
			checkDiverged()
		}
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
