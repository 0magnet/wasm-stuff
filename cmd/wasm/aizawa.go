//go:build js && wasm

package main

var aizawaDT, aizawaA, aizawaB, aizawaC, aizawaD, aizawaE, aizawaF float32 = 0.0052, 0.95, 0.7, 0.6, 3.5, 0.25, 0.1

func generateAizawa() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		x1 := x + aizawaDT*((z-aizawaB)*x-aizawaD*y)
		y1 := y + aizawaDT*(aizawaD*x+(z-aizawaB)*y)
		z1 := z + aizawaDT*(aizawaC+aizawaA*z-(z*z*z)/3-(x*x+y*y)*(1+aizawaE*z)+aizawaF*z*x*x*x)
		x, y, z = x1, y1, z1
		vertices[i*3] = x
		vertices[i*3+1] = y
		vertices[i*3+2] = z
	}
	uploadVerticesOnly(vertices, glTypes.LineStrip, len(vertices)/3)
}
