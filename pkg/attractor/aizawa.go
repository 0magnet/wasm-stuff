//go:build js && wasm

package main

var aizawaDT, aizawaA, aizawaB, aizawaC, aizawaD, aizawaE, aizawaF float32 = 0.0052, 0.95, 0.7, 0.6, 3.5, 0.25, 0.1

func generateAizawa() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := aizawaDT * speedScale
		for s := 0; s < speedSteps; s++ {
			x1 := x + dt*((z-aizawaB)*x-aizawaD*y)
			y1 := y + dt*(aizawaD*x+(z-aizawaB)*y)
			z1 := z + dt*(aizawaC+aizawaA*z-(z*z*z)/3-(x*x+y*y)*(1+aizawaE*z)+aizawaF*z*x*x*x)
			x, y, z = x1, y1, z1
			checkDiverged()
		}
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
