//go:build js && wasm

package main

var chuaDT, chuaAlpha, chuaBeta, chuaM0, chuaM1 float32 = 0.005, 15.6, 28.0, -1.143, -0.714

func generateChua() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := chuaDT * speedScale
		for s := 0; s < speedSteps; s++ {
			// h(x) = m1*x + 0.5*(m0-m1)*(|x+1| - |x-1|)
			abxp1 := x + 1
			if abxp1 < 0 {
				abxp1 = -abxp1
			}
			abxm1 := x - 1
			if abxm1 < 0 {
				abxm1 = -abxm1
			}
			hx := chuaM1*x + 0.5*(chuaM0-chuaM1)*(abxp1-abxm1)
			x1 := x + dt*chuaAlpha*(y-x-hx)
			y1 := y + dt*(x-y+z)
			z1 := z + dt*(-chuaBeta*y)
			x, y, z = x1, y1, z1
			checkDiverged()
		}
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
