//go:build js && wasm

package main

var sprottDT, sprottA, sprottB float32 = 0.01, 2.07, 1.8

func generateSprott() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		x1 := x + sprottDT*(y+sprottA*x*y+x*z)
		y1 := y + sprottDT*(1-sprottB*x*x+y*z)
		z1 := z + sprottDT*(x-x*x-y*y)
		x, y, z = x1, y1, z1
		vertices[i*3] = x
		vertices[i*3+1] = y
		vertices[i*3+2] = z
	}
	uploadVerticesOnly(vertices, glTypes.LineStrip, len(vertices)/3)
}
