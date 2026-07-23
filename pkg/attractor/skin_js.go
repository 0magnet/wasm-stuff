//go:build js && wasm

package attractor

import (
	"math"
	"syscall/js"
)

// Spectrogram "skin": paint the live spectrogram texture onto a surface
// model instead of its gradient wireframe. Enabled by a checkbox; applies
// to the parametric surfaces (sphere, globe, torus) whose UV mapping is
// natural — u = time (wraps around, scrolling), v = frequency. Flat-faced
// models (cube, polyhedra) would need per-face UV unwrapping and are left
// for later.
//
// The skinned model is a filled, UV-mapped triangle mesh drawn through
// texProgram, so it rotates/zooms/auto-rotates via the normal render path.

var (
	spectroSkin  bool // skin checkbox state
	skinDirty    = true
	skinVBuf     js.Value
	skinIBuf     js.Value
	skinIdxCount int
)

// isSkinnable reports whether the spectrogram skin can be applied to a mode.
func isSkinnable(mode string) bool {
	switch mode {
	case "sphere", "globe", "torus":
		return true
	}
	return false
}

// renderSkinnedMode keeps the spectrogram texture current and draws the
// current surface model as a filled, textured mesh. Called from
// generateForMode when the skin is on and the mode is skinnable.
func renderSkinnedMode(mode string, nowMs float64) {
	if !spectReady {
		initSpectrogram()
	}
	ensureAudioSource()
	updateSpectrogramTexture(nowMs)

	if skinDirty || skinVBuf.IsUndefined() {
		buildSkinMesh(mode)
		skinDirty = false
	}
	offset := float32(spectTexCol) / float32(spectTexW)
	drawTexturedMesh(skinVBuf, skinIBuf, skinIdxCount, spectTexture, offset)
	maybeShowAudioStatus()
}

// buildSkinMesh (re)generates and uploads the interleaved pos+uv vertex
// buffer and triangle index buffer for the current mode's surface.
func buildSkinMesh(mode string) {
	var verts []float32
	var idx []uint16
	switch mode {
	case "torus":
		verts, idx = torusSkinMesh(torusR, torusr, int(torusStacksF), int(torusSlicesF))
	case "globe":
		verts, idx = sphereSkinMesh(1.0, int(globeLatF)*2, int(globeLonF))
	default: // sphere
		verts, idx = sphereSkinMesh(sphereRadius, int(sphereStacksF), int(sphereSlicesF))
	}
	if skinVBuf.IsUndefined() {
		skinVBuf = gl.Call("createBuffer")
	}
	if skinIBuf.IsUndefined() {
		skinIBuf = gl.Call("createBuffer")
	}
	gl.Call("bindBuffer", glTypes.ArrayBuffer, skinVBuf)
	gl.Call("bufferData", glTypes.ArrayBuffer, SliceToTypedArray(verts), glTypes.StaticDraw)
	gl.Call("bindBuffer", glTypes.ElementArrayBuffer, skinIBuf)
	gl.Call("bufferData", glTypes.ElementArrayBuffer, SliceToTypedArray(idx), glTypes.StaticDraw)
	skinIdxCount = len(idx)
}

// gridTriangles emits two triangles per (stacks x slices) grid quad for a
// vertex layout of (slices+1) columns per row.
func gridTriangles(stacks, slices int) []uint16 {
	idx := make([]uint16, 0, stacks*slices*6)
	row := slices + 1
	for i := 0; i < stacks; i++ {
		for j := 0; j < slices; j++ {
			a := uint16(i*row + j)
			b := a + 1
			c := uint16((i+1)*row + j)
			d := c + 1
			idx = append(idx, a, b, c, b, d, c)
		}
	}
	return idx
}

// sphereSkinMesh returns interleaved pos(xyz)+uv verts and triangle indices
// for a UV sphere. u = longitude (wraps, time axis), v = latitude
// (frequency axis, 0 Hz at the south pole so it matches the plane).
func sphereSkinMesh(radius float32, stacks, slices int) ([]float32, []uint16) {
	verts := make([]float32, 0, (stacks+1)*(slices+1)*5)
	for i := 0; i <= stacks; i++ {
		phi := float64(i) * math.Pi / float64(stacks)
		v := 1.0 - float32(i)/float32(stacks) // i=0 (north pole) → v=1 (high freq)
		for j := 0; j <= slices; j++ {
			theta := float64(j) * 2.0 * math.Pi / float64(slices)
			x := radius * float32(math.Sin(phi)*math.Cos(theta))
			y := radius * float32(math.Sin(phi)*math.Sin(theta))
			z := radius * float32(math.Cos(phi))
			u := float32(j) / float32(slices)
			verts = append(verts, x, y, z, u, v)
		}
	}
	return verts, gridTriangles(stacks, slices)
}

// torusSkinMesh returns interleaved pos+uv verts and triangle indices for a
// torus. u = around the main ring (time, wraps), v = around the tube.
func torusSkinMesh(R, r float32, stacks, slices int) ([]float32, []uint16) {
	verts := make([]float32, 0, (stacks+1)*(slices+1)*5)
	for i := 0; i <= stacks; i++ {
		theta := float64(i) * 2.0 * math.Pi / float64(stacks)
		u := float32(i) / float32(stacks)
		for j := 0; j <= slices; j++ {
			phi := float64(j) * 2.0 * math.Pi / float64(slices)
			x := (float64(R) + float64(r)*math.Cos(phi)) * math.Cos(theta)
			y := (float64(R) + float64(r)*math.Cos(phi)) * math.Sin(theta)
			z := float64(r) * math.Sin(phi)
			v := float32(j) / float32(slices)
			verts = append(verts, float32(x), float32(y), float32(z), u, v)
		}
	}
	return verts, gridTriangles(stacks, slices)
}
