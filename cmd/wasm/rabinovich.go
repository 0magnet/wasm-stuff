//go:build js && wasm

package main

var rabDT, rabAlpha, rabGamma float32 = 0.001, 0.14, 0.10

func generateRabinovich() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		dt := rabDT * speedMult
		x1 := x + dt*(y*(z-1+x*x)+rabGamma*x)
		y1 := y + dt*(x*(3*z+1-x*x)+rabGamma*y)
		z1 := z + dt*(-2*z*(rabAlpha+x*y))
		x, y, z = x1, y1, z1
		vertices[i*3], vertices[i*3+1], vertices[i*3+2] = x, y, z
	}
	uploadVerticesOnly(vertices, attractorDrawMode, len(vertices)/3)
}
