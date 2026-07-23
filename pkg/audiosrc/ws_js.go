//go:build js && wasm

package audiosrc

import (
	"encoding/base64"
	"errors"
	"math"
	"syscall/js"
)

// WSOptions configures a WebSocket audio Source.
type WSOptions struct {
	// URL is the ws:// or wss:// endpoint streaming the audio. Empty
	// means "same origin, path /ws" — resolved from window.location at
	// connect time (ws for http, wss for https). This mirrors
	// audioprism-go's client, which always talks to /ws on its own host.
	URL string

	// SampleRate is the rate (Hz) the server records at. The server side
	// forces this (audioprism-go uses 24000), and the wire format carries
	// no rate, so the consumer has to be told. Default 24000.
	SampleRate int

	// RingSize is the number of samples retained for TimeDomain
	// snapshots. Must comfortably exceed the largest window a consumer
	// asks for (FFT size). Default 8192.
	RingSize int
}

// NewWebSocket returns a Source that receives audio from a WebSocket
// server streaming base64-encoded little-endian float32 samples — the
// exact wire format produced by audioprism-go's /ws handler
// (float32SliceToBase64String). The connection is opened immediately and
// re-established automatically 2s after any drop. Samples land in a ring
// buffer that TimeDomain snapshots the tail of, so the render loop stays
// on the same pull-based contract as the mic Source.
//
// The stream is mono (the server records a single channel); Channels()
// reports 1 and TimeDomainStereo copies the one channel to both outputs.
func NewWebSocket(opts WSOptions) Source {
	if opts.SampleRate == 0 {
		opts.SampleRate = 24000
	}
	if opts.RingSize == 0 {
		opts.RingSize = 8192
	}
	w := &wsSource{
		opts: opts,
		ring: make([]float32, opts.RingSize),
	}
	if wsCtor := js.Global().Get("WebSocket"); wsCtor.IsUndefined() {
		w.err = errors.New("WebSocket not supported in this browser")
		return w
	}
	w.url = opts.URL
	if w.url == "" {
		w.url = sameOriginWSURL()
	}
	// Reuse one message handler across reconnects (matches audioprism-go).
	w.onMsg = js.FuncOf(w.handleMessage)
	w.connect()
	return w
}

// sameOriginWSURL builds ws(s)://<host>/ws from window.location, choosing
// wss when the page itself was served over https.
func sameOriginWSURL() string {
	loc := js.Global().Get("location")
	proto := "ws"
	if loc.Get("protocol").String() == "https:" {
		proto = "wss"
	}
	return proto + "://" + loc.Get("host").String() + "/ws"
}

type wsSource struct {
	opts  WSOptions
	url   string
	ws    js.Value
	onMsg js.Func

	// ring is a fixed-size circular buffer of the most recent samples.
	// head is the index the next sample will be written to; filled counts
	// how many valid samples exist (caps at len(ring)). Reads and writes
	// both happen on the JS main thread (wasm is single-threaded, all JS
	// callbacks are serialized), so no locking is needed.
	ring   []float32
	head   int
	filled int

	reconnecting bool
	ready        bool
	closed       bool
	err          error
}

func (w *wsSource) connect() {
	if w.closed {
		return
	}
	ws := js.Global().Get("WebSocket").New(w.url)
	if ws.IsUndefined() {
		w.err = errors.New("WebSocket not supported in this browser")
		return
	}
	ws.Call("addEventListener", "open", js.FuncOf(func(js.Value, []js.Value) interface{} {
		w.reconnecting = false
		w.err = nil
		return nil
	}))
	ws.Call("addEventListener", "message", w.onMsg)
	ws.Call("addEventListener", "close", js.FuncOf(func(js.Value, []js.Value) interface{} {
		w.scheduleReconnect(2000)
		return nil
	}))
	ws.Call("addEventListener", "error", js.FuncOf(func(js.Value, []js.Value) interface{} {
		// A close event follows an error and drives the reconnect; here we
		// only surface a message for the status overlay.
		if w.err == nil {
			w.err = errors.New("websocket error connecting to " + w.url)
		}
		return nil
	}))
	w.ws = ws
}

func (w *wsSource) scheduleReconnect(delayMs int) {
	if w.reconnecting || w.closed {
		return
	}
	w.reconnecting = true
	js.Global().Call("setTimeout", js.FuncOf(func(js.Value, []js.Value) interface{} {
		w.reconnecting = false
		w.connect()
		return nil
	}), delayMs)
}

// handleMessage decodes one base64 float32 chunk and pushes it into the
// ring. Bytes are little-endian, four per sample — the inverse of the
// server's float32SliceToBase64String.
func (w *wsSource) handleMessage(_ js.Value, p []js.Value) interface{} {
	if len(p) == 0 {
		return nil
	}
	data := p[0].Get("data")
	if data.Type() != js.TypeString {
		return nil
	}
	b, err := base64.StdEncoding.DecodeString(data.String())
	if err != nil {
		return nil
	}
	n := len(b) / 4
	for i := 0; i < n; i++ {
		bits := uint32(b[i*4]) | uint32(b[i*4+1])<<8 | uint32(b[i*4+2])<<16 | uint32(b[i*4+3])<<24
		w.ring[w.head] = math.Float32frombits(bits)
		w.head++
		if w.head == len(w.ring) {
			w.head = 0
		}
		if w.filled < len(w.ring) {
			w.filled++
		}
	}
	if n > 0 {
		w.ready = true
	}
	return nil
}

// snapshot copies the most recent len(dst) samples into dst, oldest
// first. Slots with no data yet read as the ring's zero value, so an
// under-filled buffer leads with silence rather than garbage.
func (w *wsSource) snapshot(dst []float32) {
	size := len(w.ring)
	n := len(dst)
	start := w.head - n
	for i := 0; i < n; i++ {
		idx := start + i
		idx %= size
		if idx < 0 {
			idx += size
		}
		dst[i] = w.ring[idx]
	}
}

func (w *wsSource) TimeDomain(dst []float32) []float32 {
	if !w.ready || len(dst) == 0 {
		for i := range dst {
			dst[i] = 0
		}
		return dst
	}
	w.snapshot(dst)
	return dst
}

func (w *wsSource) TimeDomainStereo(l, r []float32) {
	if len(l) != len(r) {
		panic("audiosrc: TimeDomainStereo requires len(l) == len(r)")
	}
	w.TimeDomain(l)
	copy(r, l) // mono stream: right mirrors left
}

func (w *wsSource) SampleRate() int { return w.opts.SampleRate }
func (w *wsSource) Channels() int {
	if !w.ready {
		return 0
	}
	return 1
}
func (w *wsSource) Ready() bool { return w.ready && !w.closed }
func (w *wsSource) Err() error  { return w.err }

func (w *wsSource) Close() {
	if w.closed {
		return
	}
	w.closed = true
	if !w.ws.IsUndefined() {
		w.ws.Call("close")
	}
}
