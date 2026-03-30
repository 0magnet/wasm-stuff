package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	htmpl "html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"
)

var (
	//go:embed b.wasm
	goWasmData []byte
	//go:embed b-tiny.wasm
	tinygoWasmData []byte
	//go:embed wasm_exec.js
	goWasmExecJS []byte
	//go:embed tinygo_wasm_exec.js
	tinygoWasmExecJS []byte
	//go:embed index.tmpl.html
	indexHtmpl string

	webPort   int
	debugMode bool
)

func init() {
	defaultport, err := strconv.Atoi(os.Getenv("WEBPORT"))
	if err != nil {
		defaultport = 8080
	}
	runCmd.Flags().IntVarP(&webPort, "port", "p", defaultport, "port to serve on - env WEBPORT="+os.Getenv("WEBPORT"))
	runCmd.Flags().BoolVarP(&debugMode, "debug", "d", false, "enable /debug/stats profiling endpoint")
}

func main() {
	Execute()
}

// Execute executes root CLI command.
func Execute() {
	cc.Init(&cc.Config{
		RootCmd:         runCmd,
		Headings:        cc.HiBlue + cc.Bold,
		Commands:        cc.HiBlue + cc.Bold,
		CmdShortDescr:   cc.HiBlue,
		Example:         cc.HiBlue + cc.Italic,
		ExecName:        cc.HiBlue + cc.Bold,
		Flags:           cc.HiBlue + cc.Bold,
		FlagsDescr:      cc.HiBlue,
		NoExtraNewlines: true,
		NoBottomNewline: true,
	})
	if err := runCmd.Execute(); err != nil {
		log.Fatal("Failed to execute command: ", err)
	}
}

const help = "Usage:\r\n" +
	"  {{.UseLine}}{{if .HasAvailableSubCommands}}{{end}} {{if gt (len .Aliases) 0}}\r\n\r\n" +
	"{{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}\r\n\r\n" +
	"Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand)}}\r\n  " +
	"{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}\r\n\r\n" +
	"Flags:\r\n" +
	"{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}\r\n\r\n" +
	"Global Flags:\r\n" +
	"{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}\r\n\r\n"

var runCmd = &cobra.Command{
	Use:   "wasm-stuff",
	Short: "wasm attractors",
	Long: `
	┬ ┬┌─┐┌─┐┌┬┐   ┌─┐┌┬┐┬ ┬┌─┐┌─┐
	│││├─┤└─┐│││───└─┐ │ │ │├┤ ├┤
	└┴┘┴ ┴└─┘┴ ┴   └─┘ ┴ └─┘└  └  `,
	Run: func(_ *cobra.Command, _ []string) {
		wg := new(sync.WaitGroup)
		r1 := gin.New()
		r1.Use(gin.Recovery())
		r1.Use(loggingMiddleware())

		hasTinygo := len(tinygoWasmData) > 0 && len(tinygoWasmExecJS) > 0

		// Go WASM - self-contained inline base64 HTML
		r1.GET("/", func(c *gin.Context) {
			serveInlineWasm(c, htmlTemplateData{
				WasmExecJs:    htmpl.JS(goWasmExecJS),
				WasmBase64:    base64.StdEncoding.EncodeToString(goWasmData),
				Title:         "Go",
				OtherLink:     "tinygo/index.html",
				OtherLabel:    "tinygo",
				CanonicalPath: "index.html",
				Debug:         debugMode,
			})
		})
		r1.GET("/index.html", func(c *gin.Context) {
			serveInlineWasm(c, htmlTemplateData{
				WasmExecJs:    htmpl.JS(goWasmExecJS),
				WasmBase64:    base64.StdEncoding.EncodeToString(goWasmData),
				Title:         "Go",
				OtherLink:     "tinygo/index.html",
				OtherLabel:    "tinygo",
				CanonicalPath: "index.html",
				Debug:         debugMode,
			})
		})

		// Go WASM assets
		r1.GET("/wasm_exec.js", func(c *gin.Context) {
			c.Data(http.StatusOK, "application/javascript", goWasmExecJS)
		})
		r1.GET("/b.wasm", func(c *gin.Context) {
			c.Render(http.StatusOK, render.Data{ContentType: "application/wasm", Data: goWasmData})
		})

		// TinyGo WASM routes (only if tinygo wasm was compiled)
		if hasTinygo {
			r1.GET("/tinygo/", func(c *gin.Context) {
				serveInlineWasm(c, htmlTemplateData{
					WasmExecJs:    htmpl.JS(tinygoWasmExecJS),
					WasmBase64:    base64.StdEncoding.EncodeToString(tinygoWasmData),
					Title:         "TinyGo",
					OtherLink:     "../index.html",
					OtherLabel:    "go",
					CanonicalPath: "tinygo/index.html",
					Debug:         debugMode,
				})
			})
			r1.GET("/tinygo/index.html", func(c *gin.Context) {
				serveInlineWasm(c, htmlTemplateData{
					WasmExecJs:    htmpl.JS(tinygoWasmExecJS),
					WasmBase64:    base64.StdEncoding.EncodeToString(tinygoWasmData),
					Title:         "TinyGo",
					OtherLink:     "../index.html",
					OtherLabel:    "go",
					CanonicalPath: "tinygo/index.html",
					Debug:         debugMode,
				})
			})
			r1.GET("/tinygo/wasm_exec.js", func(c *gin.Context) {
				c.Data(http.StatusOK, "application/javascript", tinygoWasmExecJS)
			})
			r1.GET("/tinygo/b-tiny.wasm", func(c *gin.Context) {
				c.Render(http.StatusOK, render.Data{ContentType: "application/wasm", Data: tinygoWasmData})
			})
		}

		// Debug profiling endpoint (only when --debug flag is set)
		if debugMode {
			var latestStats json.RawMessage
			var statsMu sync.RWMutex

			r1.POST("/debug/stats", func(c *gin.Context) {
				body, err := io.ReadAll(c.Request.Body)
				if err != nil {
					c.Status(http.StatusBadRequest)
					return
				}
				statsMu.Lock()
				latestStats = body
				statsMu.Unlock()
				c.Status(http.StatusOK)
			})
			r1.GET("/debug/stats", func(c *gin.Context) {
				statsMu.RLock()
				data := latestStats
				statsMu.RUnlock()
				if data == nil {
					c.JSON(http.StatusOK, gin.H{"status": "waiting for WASM to report stats..."})
					return
				}
				c.Data(http.StatusOK, "application/json", data)
			})
		}

		wg.Add(1)
		go func() {
			fmt.Printf("listening on http://127.0.0.1:%d using gin router\n", webPort)
			fmt.Printf("  Go WASM:     http://127.0.0.1:%d/index.html\n", webPort)
			if hasTinygo {
				fmt.Printf("  TinyGo WASM: http://127.0.0.1:%d/tinygo/index.html\n", webPort)
			}
			r1.Run(fmt.Sprintf(":%d", webPort))
			wg.Done()
		}()
		wg.Wait()
	},
}

func serveInlineWasm(c *gin.Context, data htmlTemplateData) {
	tmpl, err := htmpl.New("index").Parse(indexHtmpl)
	if err != nil {
		serveError(c, fmt.Sprintf("Error parsing html template:\n%v", err))
		return
	}
	var result bytes.Buffer
	if err = tmpl.Execute(&result, map[string]interface{}{"Page": data}); err != nil {
		serveError(c, fmt.Sprintf("Could not execute html template: %v", err))
		return
	}
	c.Data(http.StatusOK, "text/html;charset=utf-8", result.Bytes())
}

func serveError(c *gin.Context, msg string) {
	fmt.Println(msg)
	c.Data(http.StatusInternalServerError, "text/html;charset=utf-8",
		[]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Error</title></head><body style='background-color: black; color: white;'><div>%s</div></body></html>`,
			strings.ReplaceAll(msg, "\n", "<br>"))))
}

type GinHandler struct{ Router *gin.Engine }

func (h *GinHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.Router.ServeHTTP(w, r) }
func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		if latency > time.Minute {
			latency = latency.Truncate(time.Second)
		}
		statusCode := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		statusCodeBackgroundColor := getBackgroundColor(statusCode)
		methodColor := getMethodColor(method)
		fmt.Printf("[WASMSTUFF] | %s |%s %3d %s| %13v | %15s | %72s |%s %-7s %s %s\n", time.Now().Format("2006/01/02 - 15:04:05"), statusCodeBackgroundColor, statusCode, resetColor(), latency, c.ClientIP(), c.Request.RemoteAddr, methodColor, method, resetColor(), path)
	}
}
func getBackgroundColor(statusCode int) string {
	switch {
	case statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices:
		return green
	case statusCode >= http.StatusMultipleChoices && statusCode < http.StatusBadRequest:
		return white
	case statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError:
		return yellow
	default:
		return red
	}
}
func getMethodColor(method string) string {
	switch method {
	case http.MethodGet:
		return blue
	case http.MethodPost:
		return cyan
	case http.MethodPut:
		return yellow
	case http.MethodDelete:
		return red
	case http.MethodPatch:
		return green
	case http.MethodHead:
		return magenta
	case http.MethodOptions:
		return white
	default:
		return reset
	}
}
func resetColor() string { return reset }

const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

type htmlTemplateData struct {
	WasmExecJs    htmpl.JS
	WasmBase64    string
	Title         string
	OtherLink     string
	OtherLabel    string
	CanonicalPath string
	Debug         bool
}
