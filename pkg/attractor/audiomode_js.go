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
	audioSource      audiosrc.Source
	audioSourceTried bool
	audioModeActive  bool // last frame we rendered an audio mode
	audioOverlay     js.Value
)

// ensureAudioSource returns the shared audio source, creating it on first
// call. Safe to call every frame — the second and subsequent calls just
// return the cached value. Returns nil only if the source can't be
// constructed at all (unlikely; a failed source still returns non-nil
// with Ready()==false and Err()!=nil).
//
// The backend is chosen from the ?audio= query param:
//   - audio=ws (or websocket): stream from a WebSocket server (the
//     audioprism-go-style /ws feed served by cmd/audiows). An optional
//     ?wsurl= overrides the endpoint; default is same-origin /ws.
//   - anything else / absent: the browser microphone via getUserMedia.
func ensureAudioSource() audiosrc.Source {
	if audioSource != nil {
		return audioSource
	}
	if audioSourceTried {
		return nil
	}
	audioSourceTried = true
	switch audioBackendKind() {
	case "ws", "websocket":
		audioSource = audiosrc.NewWebSocket(audiosrc.WSOptions{
			URL: queryParam("wsurl"),
		})
	default:
		audioSource = audiosrc.NewMic(audiosrc.MicOptions{
			Stereo: true,
		})
	}
	return audioSource
}

// audioBackendKind reads the ?audio= query param (empty when absent).
func audioBackendKind() string { return queryParam("audio") }

// queryParam returns the named URL query parameter from window.location,
// or "" if not present.
func queryParam(name string) string {
	search := js.Global().Get("location").Get("search")
	params := js.Global().Get("URLSearchParams").New(search)
	if v := params.Call("get", name); v.Truthy() {
		return v.String()
	}
	return ""
}

// isAudioMode reports whether the mode bypasses the 3D pipeline and draws
// on its own 2D program. Only the xy scope does now — the spectrogram is a
// textured plane model that goes through the normal 3D render path (so it
// rotates/zooms like other models); it's handled in generateForMode.
func isAudioMode(mode string) bool {
	return mode == "xy"
}

// isAudioSourceMode reports whether a mode needs the shared audio source
// (spectrogram or xy). Used for the mic-permission status overlay.
func isAudioSourceMode(mode string) bool {
	return mode == "spectrogram" || mode == "xy"
}

// renderAudioFrame dispatches to a bypass audio mode's renderer (xy).
// Called from renderLoop when isAudioMode(selectedMode) is true.
func renderAudioFrame(mode string) {
	switch mode {
	case "xy":
		renderXYFrame()
	}
	maybeShowAudioStatus()
}

// activateAudioMode runs once when the mode dispatch transitions from an
// attractor mode into an audio mode. It kicks off the audio source (mic
// permission prompt, or WebSocket connect) but otherwise does nothing
// heavy — the individual renderers do their own lazy shader/texture setup.
func activateAudioMode() {
	audioModeActive = true
	ensureAudioSource()
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
