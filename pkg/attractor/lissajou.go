//go:build js && wasm

package main

import "math"

var lissajouA, lissajouB, lissajouC float32 = 3, 2, 5

func generateLissajou() {
	vertices := vertBuf[:steps*4]
	invN := float32(1) / float32(steps-1)
	cycles := float32(speedSteps) * speedScale
	for i := 0; i < steps; i++ {
		t := float32(i) * (2 * math.Pi) / float32(steps) * cycles
		j := i * 4
		vertices[j] = float32(math.Sin(float64(lissajouA * t)))
		vertices[j+1] = float32(math.Sin(float64(lissajouB * t)))
		vertices[j+2] = float32(math.Sin(float64(lissajouC * t)))
		vertices[j+3] = float32(i) * invN
	}
	uploadVerticesOnly(vertices, attractorDrawMode, steps)
}
