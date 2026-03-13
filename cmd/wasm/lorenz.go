//go:build js && wasm

package main

var lorenzDT, lorenzS, lorenzR, lorenzB float32 = 0.005, 10.0, 28.0, 2.7

func generateLorenz() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		x1 := x + lorenzDT*lorenzS*(y-x)
		y1 := y + lorenzDT*(x*(lorenzR-z)-y)
		z1 := z + lorenzDT*(x*y-lorenzB*z)
		x, y, z = x1, y1, z1
		vertices[i*3] = x
		vertices[i*3+1] = y
		vertices[i*3+2] = z
	}
	uploadVerticesOnly(vertices, glTypes.LineStrip, len(vertices)/3)
}
