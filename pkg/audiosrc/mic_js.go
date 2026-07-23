//go:build js && wasm

package audiosrc

import (
	"errors"
	"syscall/js"
	"unsafe"
)

// MicOptions configures a microphone Source.
type MicOptions struct {
	// FFTSize is the AnalyserNode's fftSize (must be a power of two
	// between 32 and 32768). It sets the window of samples snapshotted
	// per TimeDomain call. Default 2048 (~43 ms at 48 kHz).
	FFTSize int

	// Stereo requests a two-channel MediaStream. Falls back to mono if
	// the device or browser only exposes one channel; check Channels()
	// after Ready to know what you actually got.
	Stereo bool
}

// NewMic requests microphone access via navigator.mediaDevices.getUserMedia
// and returns a Source that reports samples via AnalyserNode(s) hanging off
// the resulting MediaStream. The call returns immediately; permission grant
// and audio graph setup happen asynchronously — poll Ready()/Err() from the
// render loop.
//
// getUserMedia requires a secure context (https or localhost). On file://
// or plain http:// LAN pages, Err() will be set once the browser refuses.
// Callers should render a fallback (e.g. a "Click to enable mic" overlay).
func NewMic(opts MicOptions) Source {
	if opts.FFTSize == 0 {
		opts.FFTSize = 2048
	}
	m := &micSource{opts: opts}
	nav := js.Global().Get("navigator")
	if nav.IsUndefined() || nav.Get("mediaDevices").IsUndefined() {
		m.err = errors.New("navigator.mediaDevices unavailable (need https or localhost)")
		return m
	}
	constraints := map[string]interface{}{
		"audio": map[string]interface{}{
			"channelCount":     boolTernary(opts.Stereo, 2, 1),
			"echoCancellation": false,
			"noiseSuppression": false,
			"autoGainControl":  false,
		},
		"video": false,
	}
	promise := nav.Get("mediaDevices").Call("getUserMedia", constraints)
	promise.Call("then", js.FuncOf(m.onStream)).Call("catch", js.FuncOf(m.onError))
	return m
}

func boolTernary(b bool, t, f int) int {
	if b {
		return t
	}
	return f
}

// micSource holds the audio graph nodes and per-channel JS Float32Array
// buffers. Buffers are allocated once and reused every TimeDomain call to
// keep the render loop off the GC path.
type micSource struct {
	opts MicOptions
	err  error

	stream    js.Value
	audioCtx  js.Value
	src       js.Value // MediaStreamAudioSourceNode
	splitter  js.Value // ChannelSplitterNode (stereo only)
	analyserL js.Value
	analyserR js.Value // undefined when mono

	jsBufL js.Value // Float32Array of length opts.FFTSize
	jsBufR js.Value

	// scratchU8 is a Uint8Array view onto the ArrayBuffer backing jsBufL
	// (and separately jsBufR). CopyBytesToGo copies bytes from JS to Go
	// in one call; we reinterpret the bytes as float32 via unsafe.
	scratchU8L js.Value
	scratchU8R js.Value
	byteBufL   []byte
	byteBufR   []byte

	sampleRate int
	channels   int
	ready      bool
	closed     bool
}

func (m *micSource) onStream(_ js.Value, args []js.Value) interface{} {
	if m.closed {
		if len(args) > 0 {
			stopTracks(args[0])
		}
		return nil
	}
	m.stream = args[0]

	ac := js.Global().Get("AudioContext")
	if ac.IsUndefined() {
		ac = js.Global().Get("webkitAudioContext")
	}
	if ac.IsUndefined() {
		m.err = errors.New("AudioContext unavailable")
		stopTracks(m.stream)
		return nil
	}
	m.audioCtx = ac.New()
	m.sampleRate = m.audioCtx.Get("sampleRate").Int()

	m.src = m.audioCtx.Call("createMediaStreamSource", m.stream)

	// Detect actual channel count from the stream's audio track.
	tracks := m.stream.Call("getAudioTracks")
	if tracks.Length() > 0 {
		settings := tracks.Index(0).Call("getSettings")
		if v := settings.Get("channelCount"); !v.IsUndefined() {
			m.channels = v.Int()
		}
	}
	if m.channels == 0 {
		m.channels = 1
	}

	m.analyserL = m.audioCtx.Call("createAnalyser")
	m.analyserL.Set("fftSize", m.opts.FFTSize)
	m.analyserL.Set("smoothingTimeConstant", 0)

	if m.channels >= 2 {
		m.splitter = m.audioCtx.Call("createChannelSplitter", 2)
		m.src.Call("connect", m.splitter)
		m.splitter.Call("connect", m.analyserL, 0)
		m.analyserR = m.audioCtx.Call("createAnalyser")
		m.analyserR.Set("fftSize", m.opts.FFTSize)
		m.analyserR.Set("smoothingTimeConstant", 0)
		m.splitter.Call("connect", m.analyserR, 1)
	} else {
		m.src.Call("connect", m.analyserL)
	}

	m.jsBufL = js.Global().Get("Float32Array").New(m.opts.FFTSize)
	m.scratchU8L = js.Global().Get("Uint8Array").New(m.jsBufL.Get("buffer"))
	m.byteBufL = make([]byte, m.opts.FFTSize*4)
	if m.channels >= 2 {
		m.jsBufR = js.Global().Get("Float32Array").New(m.opts.FFTSize)
		m.scratchU8R = js.Global().Get("Uint8Array").New(m.jsBufR.Get("buffer"))
		m.byteBufR = make([]byte, m.opts.FFTSize*4)
	}

	m.ready = true
	return nil
}

func (m *micSource) onError(_ js.Value, args []js.Value) interface{} {
	msg := "mic permission denied"
	if len(args) > 0 {
		if name := args[0].Get("name"); !name.IsUndefined() {
			msg = name.String() + ": " + args[0].Get("message").String()
		}
	}
	m.err = errors.New(msg)
	return nil
}

func (m *micSource) TimeDomain(dst []float32) []float32 {
	if !m.ready || len(dst) == 0 {
		for i := range dst {
			dst[i] = 0
		}
		return dst
	}
	m.readAnalyser(m.analyserL, m.jsBufL, m.scratchU8L, m.byteBufL, dst)
	return dst
}

func (m *micSource) TimeDomainStereo(l, r []float32) {
	if len(l) != len(r) {
		panic("audiosrc: TimeDomainStereo requires len(l) == len(r)")
	}
	if !m.ready {
		for i := range l {
			l[i] = 0
			r[i] = 0
		}
		return
	}
	m.readAnalyser(m.analyserL, m.jsBufL, m.scratchU8L, m.byteBufL, l)
	if m.channels >= 2 {
		m.readAnalyser(m.analyserR, m.jsBufR, m.scratchU8R, m.byteBufR, r)
	} else {
		copy(r, l)
	}
}

// readAnalyser pulls the AnalyserNode's most-recent-samples window into the
// supplied JS Float32Array, then copies its bytes into a Go slice we
// reinterpret as float32. The fftSize we set at creation determines how
// many samples the buffer holds; if the caller wants fewer we truncate,
// more and we zero-fill the tail.
func (m *micSource) readAnalyser(analyser, jsBuf, jsU8 js.Value, byteBuf []byte, out []float32) {
	analyser.Call("getFloatTimeDomainData", jsBuf)
	js.CopyBytesToGo(byteBuf, jsU8)
	// Reinterpret the byte buffer as []float32. Same length in floats as
	// the JS Float32Array we allocated (opts.FFTSize). Native byte order
	// on wasm is little-endian; Float32Array is host-native, so no swap.
	src := unsafe.Slice((*float32)(unsafe.Pointer(&byteBuf[0])), len(byteBuf)/4)
	n := len(out)
	if n > len(src) {
		copy(out, src)
		for i := len(src); i < n; i++ {
			out[i] = 0
		}
		return
	}
	// Take the last n samples so callers requesting a window smaller
	// than the FFTSize get the most recent audio (AnalyserNode fills
	// the buffer with the most recent fftSize samples, oldest first).
	copy(out, src[len(src)-n:])
}

func (m *micSource) SampleRate() int { return m.sampleRate }
func (m *micSource) Channels() int   { return m.channels }
func (m *micSource) Ready() bool     { return m.ready && !m.closed }
func (m *micSource) Err() error      { return m.err }

func (m *micSource) Close() {
	if m.closed {
		return
	}
	m.closed = true
	if !m.stream.IsUndefined() {
		stopTracks(m.stream)
	}
	if !m.audioCtx.IsUndefined() {
		m.audioCtx.Call("close")
	}
}

// stopTracks stops every audio track on a MediaStream so the browser mic
// indicator turns off promptly.
func stopTracks(stream js.Value) {
	if stream.IsUndefined() || stream.IsNull() {
		return
	}
	tracks := stream.Call("getTracks")
	n := tracks.Length()
	for i := 0; i < n; i++ {
		tracks.Index(i).Call("stop")
	}
}
