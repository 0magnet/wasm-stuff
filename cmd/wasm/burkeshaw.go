//go:build js && wasm

package main

var burkeDT, burkeS, burkeV float32 = 0.005, 10.0, 4.272

func generateBurkeShaw() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		dt := burkeDT * speedMult
		x1 := x + dt*(-burkeS*(x+y))
		y1 := y + dt*(-y-burkeS*x*z)
		z1 := z + dt*(burkeS*x*y+burkeV)
		x, y, z = x1, y1, z1
		vertices[i*3], vertices[i*3+1], vertices[i*3+2] = x, y, z
	}
	uploadVerticesOnly(vertices, attractorDrawMode, len(vertices)/3)
}
