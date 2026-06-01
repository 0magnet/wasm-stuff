//go:build js && wasm

package main

var rabDT, rabAlpha, rabGamma float32 = 0.001, 0.14, 0.10

func generateRabinovich() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := rabDT * speedScale
		for s := 0; s < speedSteps; s++ {
			x1 := x + dt*(y*(z-1+x*x)+rabGamma*x)
			y1 := y + dt*(x*(3*z+1-x*x)+rabGamma*y)
			z1 := z + dt*(-2*z*(rabAlpha+x*y))
			x, y, z = x1, y1, z1
			checkDiverged()
		}
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
