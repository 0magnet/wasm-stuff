//go:build js && wasm

package attractor

import (
	"syscall/js"

	"github.com/0magnet/wasm-stuff/pkg/audiosrc"
)

// Shared plumbing for audio-driven modes (spectrogram, xy scope, and
// eventually audio-modulated attractors). The audiosrc.Source is created
// lazily on first entry into any audio mode and kept alive across mode
// switches so the mic permission prompt only appears once. It's also
// safe to have live when the user is in an attractor mode — no cost, no
// CPU work, just an idle AudioContext.

var (
	audioSource     audiosrc.Source
	audioSourceTried bool
	audioModeActive bool // last frame we rendered an audio mode
	audioOverlay    js.Value
)

// ensureMicSource returns the shared audio source, creating it on first
// call. Safe to call every frame — the second and subsequent calls just
// return the cached value. Returns nil only if the source can't be
// constructed at all (unlikely; permission-denied still returns a
// non-nil source with Ready()==false and Err()!=nil).
func ensureMicSource() audiosrc.Source {
	if audioSource != nil {
		return audioSource
	}
	if audioSourceTried {
		return nil
	}
	audioSourceTried = true
	audioSource = audiosrc.NewMic(audiosrc.MicOptions{
		FFTSize: 2048,
		Stereo:  true,
	})
	return audioSource
}

// isAudioMode reports whether the given mode name is one of the
// audio-driven visualizations (spectrogram, xy). Called each frame in
// renderLoop to decide the dispatch branch.
func isAudioMode(mode string) bool {
	switch mode {
	case "spectrogram", "xy":
		return true
	}
	return false
}

// renderAudioFrame dispatches to the current audio mode's renderer.
// Called from renderLoop when isAudioMode(selectedMode) is true.
func renderAudioFrame(mode string) {
	switch mode {
	case "spectrogram":
		renderSpectrogramFrame()
	case "xy":
		renderXYFrame()
	}
	maybeShowAudioStatus()
}

// activateAudioMode runs once when the mode dispatch transitions from an
// attractor mode into an audio mode. It kicks off the mic request (which
// pops the browser permission prompt) but otherwise does nothing heavy —
// the individual renderers do their own lazy shader/texture setup.
func activateAudioMode() {
	audioModeActive = true
	ensureMicSource()
}

// deactivateAudioMode runs once when we transition back out of an audio
// mode. Restores the attractor pipeline: rebinds the attractor shader
// program and marks static geometry dirty so the next indexed upload
// re-sets attribute pointers (audio modes leave their own attribute
// pointers active on aPos).
func deactivateAudioMode() {
	audioModeActive = false
	if !shaderProgram.IsUndefined() {
		gl.Call("useProgram", shaderProgram)
	}
	staticGeomDirty = true
	if audioOverlay.Truthy() {
		audioOverlay.Get("style").Set("display", "none")
	}
}

// maybeShowAudioStatus creates a small overlay message when the mic
// isn't Ready yet (permission still pending, or denied). The overlay
// lives at the top-center of the canvas. Hidden once the source starts
// producing samples, or when we leave audio mode.
func maybeShowAudioStatus() {
	if audioSource == nil {
		return
	}
	if audioSource.Ready() && audioSource.Err() == nil {
		if audioOverlay.Truthy() {
			audioOverlay.Get("style").Set("display", "none")
		}
		return
	}
	if !audioOverlay.Truthy() {
		audioOverlay = doc.Call("createElement", "div")
		audioOverlay.Set("id", "audio-status-overlay")
		style := audioOverlay.Get("style")
		style.Set("position", "fixed")
		style.Set("top", "20px")
		style.Set("left", "50%")
		style.Set("transform", "translateX(-50%)")
		style.Set("padding", "8px 14px")
		style.Set("background", "rgba(0,0,0,0.7)")
		style.Set("color", "#fff")
		style.Set("font-family", "monospace")
		style.Set("font-size", "13px")
		style.Set("border", "1px solid #555")
		style.Set("z-index", "50")
		style.Set("pointer-events", "none")
		body.Call("appendChild", audioOverlay)
	}
	msg := "Requesting microphone access..."
	if err := audioSource.Err(); err != nil {
		msg = "Mic unavailable: " + err.Error()
	}
	audioOverlay.Set("textContent", msg)
	audioOverlay.Get("style").Set("display", "block")
}
