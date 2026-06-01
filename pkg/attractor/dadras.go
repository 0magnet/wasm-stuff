//go:build js && wasm

package main

var dadrasDT, dadrasP, dadrasQ, dadrasR, dadrasS, dadrasE float32 = 0.005, 3.0, 2.7, 1.7, 2.0, 9.0

func generateDadras() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := dadrasDT * speedScale
		for s := 0; s < speedSteps; s++ {
			x1 := x + dt*(y-dadrasP*x+dadrasQ*y*z)
			y1 := y + dt*(dadrasR*y-x*z+z)
			z1 := z + dt*(dadrasS*x*y-dadrasE*z)
			x, y, z = x1, y1, z1
			checkDiverged()
		}
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
