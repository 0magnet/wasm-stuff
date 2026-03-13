//go:build js && wasm

package main

import "math"

var lissajouA, lissajouB, lissajouC float32 = 9, 4, 25

func generateLissajou() {
	vertices := vertBuf[:steps*3]
	for i := 0; i < steps; i++ {
		t := float32(i) * (2 * math.Pi) / float32(steps)
		vertices[i*3] = float32(math.Sin(float64(lissajouA * t)))
		vertices[i*3+1] = float32(math.Sin(float64(lissajouB * t)))
		vertices[i*3+2] = float32(math.Sin(float64(lissajouC * t)))
	}
	uploadVerticesOnly(vertices, glTypes.LineStrip, len(vertices)/3)
}
