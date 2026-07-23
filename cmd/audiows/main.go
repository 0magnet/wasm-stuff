// Command audiows is a development/test harness that streams live system
// audio to the wasm-stuff browser build over a WebSocket, and serves the
// wasm page on the same origin so the whole thing works from one process.
//
// It mirrors audioprism-go's coreweb/wasm server: PulseAudio captures the
// default source, and each chunk of float32 samples is base64-encoded
// (little-endian, four bytes per sample) and pushed over /ws. The
// browser-side WebSocket audiosrc.Source (pkg/audiosrc/ws_js.go) decodes
// the identical wire format, so the spectrogram and xy-scope modes behave
// the same here as they do in audioprism-go's wasm build.
//
// Usage:
//
//	make build            # produce b.wasm / wasm_exec.js / index.tmpl.html
//	go run ./cmd/audiows  # serve on :8080, capturing the default PA source
//	# open http://127.0.0.1:8080/  → redirects to /?audio=ws
//
// Requires a running PulseAudio (or PipeWire-pulse) server. This binary is
// Linux-oriented and intentionally kept out of the portable root server so
// that `wasm-stuff` stays free of the audio-capture dependency.
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	htmpl "html/template"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/jfreymuth/pulse"
	"golang.org/x/net/websocket"
)

var (
	webPort    int
	assetDir   string
	sampleRate int
	latency    float64
)

func init() {
	flag.IntVar(&webPort, "port", 8080, "port to serve on")
	flag.IntVar(&webPort, "p", 8080, "port to serve on (shorthand)")
	flag.StringVar(&assetDir, "dir", ".", "directory holding b.wasm, wasm_exec.js and index.tmpl.html")
	flag.StringVar(&assetDir, "d", ".", "asset directory (shorthand)")
	flag.IntVar(&sampleRate, "rate", 24000, "PulseAudio record sample rate (Hz)")
	flag.Float64Var(&latency, "latency", 0.1, "PulseAudio record latency (seconds)")
}

type htmlTemplateData struct {
	WasmExecJs    htmpl.JS
	WasmBase64    string
	Title         string
	OtherLink     string
	OtherLabel    string
	CanonicalPath string
	Debug         bool
}

func main() {
	flag.Parse()

	wasmExecJS, err := os.ReadFile(filepath.Join(assetDir, "wasm_exec.js"))
	if err != nil {
		log.Fatalf("read wasm_exec.js (run `make build` first, or pass -dir): %v", err)
	}
	wasmData, err := os.ReadFile(filepath.Join(assetDir, "b.wasm"))
	if err != nil {
		log.Fatalf("read b.wasm (run `make build` first, or pass -dir): %v", err)
	}
	tmplSrc, err := os.ReadFile(filepath.Join(assetDir, "index.tmpl.html"))
	if err != nil {
		log.Fatalf("read index.tmpl.html (run from repo root, or pass -dir): %v", err)
	}
	tmpl, err := htmpl.New("index").Parse(string(tmplSrc))
	if err != nil {
		log.Fatalf("parse index.tmpl.html: %v", err)
	}

	page := htmlTemplateData{
		WasmExecJs:    htmpl.JS(wasmExecJS), //nolint:gosec // local dev asset
		WasmBase64:    base64.StdEncoding.EncodeToString(wasmData),
		Title:         "Go",
		OtherLink:     "index.html",
		OtherLabel:    "go",
		CanonicalPath: "index.html",
		Debug:         false,
	}

	serveIndex := func(c *gin.Context) {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, map[string]interface{}{"Page": page}); err != nil {
			c.String(http.StatusInternalServerError, "template error: %v", err)
			return
		}
		c.Data(http.StatusOK, "text/html;charset=utf-8", buf.Bytes())
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Land on the ws backend automatically, matching audioprism-go's
	// zero-config auto-connect. The wasm reads ?audio=ws to pick the
	// WebSocket source over the default microphone source.
	r.GET("/", func(c *gin.Context) {
		if c.Query("audio") == "" {
			c.Redirect(http.StatusFound, "/?audio=ws")
			return
		}
		serveIndex(c)
	})
	r.GET("/index.html", serveIndex)
	r.GET("/wasm_exec.js", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/javascript", wasmExecJS)
	})
	r.GET("/b.wasm", func(c *gin.Context) {
		c.Render(http.StatusOK, render.Data{ContentType: "application/wasm", Data: wasmData})
	})
	r.GET("/ws", func(c *gin.Context) {
		websocket.Handler(wsHandler).ServeHTTP(c.Writer, c.Request)
	})

	addr := fmt.Sprintf(":%d", webPort)
	log.Printf("audiows: serving http://127.0.0.1:%d/ (audio via PulseAudio @ %d Hz)", webPort, sampleRate)
	log.Printf("audiows: streaming base64 float32 over ws://127.0.0.1:%d/ws", webPort)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

// wsHandler opens a PulseAudio record stream and forwards every chunk to
// the browser as a base64 string, for the lifetime of the WebSocket. One
// PulseAudio client per connection keeps the failure of one browser tab
// from taking down the others (unlike a shared client). Errors are logged
// and end this connection only — never the whole server.
func wsHandler(ws *websocket.Conn) {
	defer ws.Close()

	client, err := pulse.NewClient()
	if err != nil {
		log.Printf("audiows: pulse.NewClient: %v (is PulseAudio/PipeWire running?)", err)
		return
	}
	defer client.Close()

	stream, err := client.NewRecord(pulse.Float32Writer(func(p []float32) (int, error) {
		if len(p) == 0 {
			return 0, nil
		}
		if err := websocket.Message.Send(ws, float32SliceToBase64(p)); err != nil {
			return 0, err // closed connection — unwinds the record stream
		}
		return len(p), nil
	}), pulse.RecordSampleRate(sampleRate), pulse.RecordLatency(latency))
	if err != nil {
		log.Printf("audiows: NewRecord: %v", err)
		return
	}

	stream.Start()
	defer stream.Stop()

	// Block until the client goes away. The browser never sends data; a
	// receive error means the socket closed, so we can tear down.
	for {
		var msg string
		if err := websocket.Message.Receive(ws, &msg); err != nil {
			return
		}
	}
}

// float32SliceToBase64 encodes samples as little-endian float32 bytes then
// base64 — byte-for-byte identical to audioprism-go's server, so the
// browser decoder in pkg/audiosrc/ws_js.go reads it unchanged.
func float32SliceToBase64(floats []float32) string {
	b := make([]byte, len(floats)*4)
	for i, f := range floats {
		bits := math.Float32bits(f)
		b[i*4+0] = byte(bits >> 0)
		b[i*4+1] = byte(bits >> 8)
		b[i*4+2] = byte(bits >> 16)
		b[i*4+3] = byte(bits >> 24)
	}
	return base64.StdEncoding.EncodeToString(b)
}
