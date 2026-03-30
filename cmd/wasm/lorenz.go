//go:build js && wasm

package main

var lorenzDT, lorenzS, lorenzR, lorenzB float32 = 0.005, 10.0, 28.0, 2.7

func generateLorenz() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := lorenzDT * speedScale
		for s := 0; s < speedSteps; s++ {
			x1 := x + dt*lorenzS*(y-x)
			y1 := y + dt*(x*(lorenzR-z)-y)
			z1 := z + dt*(x*y-lorenzB*z)
			x, y, z = x1, y1, z1
			checkDiverged()
		}
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
