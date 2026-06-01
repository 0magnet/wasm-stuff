//go:build js && wasm

package attractor

var chenDT, chenA, chenB, chenC float32 = 0.0005, 35.0, 3.0, 28.0

func generateChen() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	for i := 0; i < steps; i++ {
		dt := chenDT * speedScale
		for s := 0; s < speedSteps; s++ {
			x1 := x + dt*chenA*(y-x)
			y1 := y + dt*((chenC-chenA)*x-x*z+chenC*y)
			z1 := z + dt*(x*y-chenB*z)
			x, y, z = x1, y1, z1
			checkDiverged()
		}
		j := i * 4
		vertices[j], vertices[j+1], vertices[j+2], vertices[j+3] = x, y, z, float32(i)*invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
