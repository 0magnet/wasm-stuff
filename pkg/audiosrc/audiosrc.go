// Package audiosrc provides browser audio sources for wasm-stuff
// visualizations that want live audio input (spectrogram, xy scope, and
// eventually audio-modulated attractor parameters).
//
// Sources are pull-based: consumers call TimeDomain(dst) once per
// animation frame to snapshot the most recent samples. This suits the
// requestAnimationFrame render loop and sidesteps the goroutine/callback
// complexity of a push-based design. Actual audio scheduling stays inside
// the browser's AudioContext where it belongs.
//
// All implementations are JS/WASM-only. This file has no build tag so
// non-WASM consumers can still refer to the Source type in shared code
// (e.g. mode dispatch); constructors live in *_js.go.
package audiosrc

// Source is a live audio input feeding time-domain samples in [-1, 1].
//
// Sources may take an unbounded amount of time to become Ready (mic
// permission prompt, WebSocket handshake, etc.). Consumers should call
// Ready() before trusting TimeDomain output; before that, the destination
// slice is left zero-filled.
type Source interface {
	// TimeDomain fills dst with the most recent len(dst) samples from
	// the primary channel and returns dst. If the source is not Ready,
	// dst is zeroed.
	TimeDomain(dst []float32) []float32

	// TimeDomainStereo fills l and r with the most recent samples from
	// the left and right channels respectively. len(l) must equal
	// len(r). If the underlying source is mono, r receives a copy of l
	// (callers wanting a pseudo-stereo Lissajous can post-process with
	// a lag). Not Ready → both zeroed.
	TimeDomainStereo(l, r []float32)

	// SampleRate returns the underlying AudioContext's sampleRate in
	// Hz (typically 44100 or 48000). Returns 0 before the context is
	// established.
	SampleRate() int

	// Channels reports the number of audio channels the source is
	// actually delivering (1 or 2). Returns 0 before Ready.
	Channels() int

	// Ready reports whether the source has been fully initialized and
	// is delivering samples. Mic sources are async (user permission);
	// they start not-Ready and flip once permission is granted.
	Ready() bool

	// Err returns any terminal error that has occurred (e.g., user
	// denied mic permission, WebSocket refused). Nil while the source
	// is either still initializing or working normally.
	Err() error

	// Close releases browser resources: stops MediaStream tracks,
	// disconnects AudioNodes, closes WebSockets. Safe to call multiple
	// times or before Ready.
	Close()
}
