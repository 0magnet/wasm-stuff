//go:build js && wasm

package main

var chuaDT, chuaA, chuaB, chuaC float32 = 0.005, 40, 3.0, 28.0

func generateChua() {
	vertices := vertBuf[:steps*3]
	lx, ly, lz := float32(0.1), float32(0), float32(0)
	for i := 0; i < steps; i++ {
		dt := chuaDT * speedMult
		x1 := lx + dt*chuaA*(ly-lx-chuaB*lz)
		y1 := ly + dt*(lx-lx*ly-lz)
		z1 := lz + dt*(chuaB*ly-lz)
		lx, ly, lz = x1, y1, z1
		vertices[i*3], vertices[i*3+1], vertices[i*3+2] = lx, ly, lz
	}
	uploadVerticesOnly(vertices, attractorDrawMode, len(vertices)/3)
}
