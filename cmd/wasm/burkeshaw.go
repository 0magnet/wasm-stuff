//go:build js && wasm

package main

var burkeDT, burkeS, burkeV float32 = 0.005, 10.0, 4.272

func generateBurkeShaw() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := burkeDT * speedMult
		x1 := x + dt*(-burkeS*(x+y))
		y1 := y + dt*(-y-burkeS*x*z)
		z1 := z + dt*(burkeS*x*y+burkeV)
		x, y, z = x1, y1, z1
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
