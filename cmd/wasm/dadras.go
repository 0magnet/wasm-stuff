//go:build js && wasm

package main

var dadrasDT, dadrasP, dadrasQ, dadrasR, dadrasS, dadrasE float32 = 0.005, 3.0, 2.7, 1.7, 2.0, 9.0

func generateDadras() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		dt := dadrasDT * speedMult
		x1 := x + dt*(y-dadrasP*x+dadrasQ*y*z)
		y1 := y + dt*(dadrasR*y-x*z+z)
		z1 := z + dt*(dadrasS*x*y-dadrasE*z)
		x, y, z = x1, y1, z1
		vertices[i*3], vertices[i*3+1], vertices[i*3+2] = x, y, z
	}
	uploadVerticesOnly(vertices, attractorDrawMode, len(vertices)/3)
}
