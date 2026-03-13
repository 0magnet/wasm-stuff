//go:build js && wasm

package main

var chuaDT, chuaA, chuaB, chuaC float32 = 0.005, 40, 3.0, 28.0

func generateChua() {
	vertices := vertBuf[:steps*3]
	lx, ly, lz := float32(0.1), float32(0), float32(0)
	for i := 0; i < steps; i++ {
		x1 := lx + chuaDT*chuaA*(ly-lx-chuaB*lz)
		y1 := ly + chuaDT*(lx-lx*ly-lz)
		z1 := lz + chuaDT*(chuaB*ly-lz)
		lx, ly, lz = x1, y1, z1
		vertices[i*3] = lx
		vertices[i*3+1] = ly
		vertices[i*3+2] = lz
	}
	uploadVerticesOnly(vertices, glTypes.LineStrip, len(vertices)/3)
}
