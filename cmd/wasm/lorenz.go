//go:build js && wasm

package main

var lorenzDT, lorenzS, lorenzR, lorenzB float32 = 0.005, 10.0, 28.0, 2.7

func generateLorenz() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		dt := lorenzDT * speedMult
		x1 := x + dt*lorenzS*(y-x)
		y1 := y + dt*(x*(lorenzR-z)-y)
		z1 := z + dt*(x*y-lorenzB*z)
		x, y, z = x1, y1, z1
		vertices[i*3], vertices[i*3+1], vertices[i*3+2] = x, y, z
	}
	uploadVerticesOnly(vertices, attractorDrawMode, len(vertices)/3)
}
