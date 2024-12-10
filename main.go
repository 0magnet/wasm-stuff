package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	htmpl "html/template"
	"time"

	"github.com/bitfield/script"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/spf13/cobra"
	cc "github.com/ivanpirog/coloredcobra")

var (
	webPort int
)
func init() {
defaultport, err := strconv.Atoi(os.Getenv("WEBPORT"))
if err != nil {	defaultport = 8080 }
runCmd.Flags().IntVarP(&webPort, "port", "p", defaultport, "port to serve on - env WEBPORT="+os.Getenv("WEBPORT"))
}
func main() {
	_, err = script.Exec(`go help`).Bytes()
	if err != nil {
		log.Fatal("error on golang invocation: ", err)
	}

	Execute()
}

// Execute executes root CLI command.
func Execute() {
	cc.Init(&cc.Config{
		RootCmd:       runCmd,
		Headings:      cc.HiBlue + cc.Bold,
		Commands:      cc.HiBlue + cc.Bold,
		CmdShortDescr: cc.HiBlue,
		Example:       cc.HiBlue + cc.Italic,
		ExecName:      cc.HiBlue + cc.Bold,
		Flags:         cc.HiBlue + cc.Bold,
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

var wasmExecLocation = runtime.GOROOT() + "/misc/wasm/wasm_exec.js"
var tinygowasmExecLocation = strings.TrimSuffix(runtime.GOROOT(), "go") + "tinygo" + "/targets/wasm_exec.js"
var wasmData []byte
var htmlPageTemplateData htmlTemplateData
var tmpl *htmpl.Template
var err error
var runCmd = &cobra.Command{
Use:   "wasm-stuff",
Short: "wasm attractors",
Long: `
	┬ ┬┌─┐┌─┐┌┬┐   ┌─┐┌┬┐┬ ┬┌─┐┌─┐
	│││├─┤└─┐│││───└─┐ │ │ │├┤ ├┤
	└┴┘┴ ┴└─┘┴ ┴   └─┘ ┴ └─┘└  └  `,
Run: func(_ *cobra.Command, _ []string) {
wg := new(sync.WaitGroup)
wasmExecData, err := script.File(wasmExecLocation).Bytes()
if err != nil {			log.Fatal("Could not read wasm_exec.js file: ", err)		}
r1 := gin.New()
r1.Use(gin.Recovery())
r1.Use(loggingMiddleware())
r1.GET("/index.html", func(c *gin.Context) {
	c.Writer.Header().Set("Server", "")
	c.Writer.Header().Set("Content-Type", "text/html;charset=utf-8")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()
	tmpl, err = htmpl.New("index").Parse(indexHtmpl)
	if err != nil {
		msg := fmt.Sprintf("Error parsing html template indexHtmpl:\n%s\n%v\n", indexHtmpl, err)
		fmt.Println(msg)
		c.Writer.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Error</title></head><body style='background-color: black; color: white;'><div>%s</div></body></html>`, strings.ReplaceAll(msg, "\n", "<br>"))))
		c.Writer.Flush()
		return
	}
	wasmExecScript, err := script.File(wasmExecLocation).String()
	if err != nil {			log.Fatal("Could not read wasm_exec.js file: %s\n", err)		}
	htmlPageTemplateData.WasmExecJs  = htmpl.JS(wasmExecScript)
	wasmData, err = script.Exec(`bash -c 'GOOS=js GOARCH=wasm go build -o /dev/stdout b.go'`).Bytes()
	if err != nil {
		msg := fmt.Sprintf("Could not compile or read wasm file:\n%s\n%v\n", string(wasmData), err)
		fmt.Println(msg)
		c.Writer.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Error</title></head><body style='background-color: black; color: white;'><div>%s</div></body></html>`, strings.ReplaceAll(msg, "\n", "<br>"))))
		c.Writer.Flush()
		return
	}
	htmlPageTemplateData.WasmBase64  = base64.StdEncoding.EncodeToString(wasmData)
	tmplData := map[string]interface{}{
		"Page":  htmlPageTemplateData,
	}
	var result bytes.Buffer
	err = tmpl.Execute(&result, tmplData)
	if err != nil {
		msg := fmt.Sprintf("Could not execute html template %v\n", err)
		fmt.Println(msg)
		c.Writer.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Error</title></head><body style='background-color: black; color: white;'><div>%s</div></body></html>`, strings.ReplaceAll(msg, "\n", "<br>"))))
		c.Writer.Flush()
		return
	}
	c.Writer.Write(result.Bytes())
	c.Writer.Flush()
})

r1.GET("/", func(c *gin.Context) {
c.Writer.Header().Set("Server", "")
c.Writer.Header().Set("Content-Type", "text/html;charset=utf-8")
c.Writer.Header().Set("Transfer-Encoding", "chunked")
c.Writer.WriteHeader(http.StatusOK)
c.Writer.Flush()
var indexH []byte
indexH, err = script.File("index0.html").Bytes()
if err != nil {
msg := fmt.Sprintf("Error reading index.html:\n%s\n%v\n", string(indexH), err)
fmt.Println(msg)
c.Writer.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Error</title></head><body style='background-color: black; color: white;'><div>%s</div></body></html>`, strings.ReplaceAll(msg, "\n", "<br>"))))
c.Writer.Flush()
return
}
wasmData, err = script.Exec(`bash -c 'GOOS=js GOARCH=wasm go build -o /dev/stdout b.go'`).Bytes()
if err != nil {
msg := fmt.Sprintf("Could not compile or read wasm file:\n%s\n%v\n", string(wasmData), err)
fmt.Println(msg)
c.Writer.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Error</title></head><body style='background-color: black; color: white;'><div>%s</div></body></html>`, strings.ReplaceAll(msg, "\n", "<br>"))))
c.Writer.Flush()
return
}
c.Writer.Write(indexH)
c.Writer.Flush()
})

r1.GET("/wasm_exec.js", func(c *gin.Context) {
c.Writer.WriteHeader(http.StatusOK)
c.Writer.Write(wasmExecData)
})
r1.GET("/bundle.wasm", func(c *gin.Context) {
c.Render(
http.StatusOK, render.Data{
ContentType: "application/wasm",
Data:        []byte(wasmData),
})
})

_, err = script.Exec(`tinygo help`).Bytes()
if err != nil {
	log.Println("error on tinygo invocation:  ", err)
} else{

r1.GET("/tinygo/index.html", func(c *gin.Context) {
	c.Writer.Header().Set("Server", "")
	c.Writer.Header().Set("Content-Type", "text/html;charset=utf-8")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()
	tmpl, err = htmpl.New("index").Parse(indexHtmpl)
	if err != nil {
		msg := fmt.Sprintf("Error parsing html template indexHtmpl:\n%s\n%v\n", indexHtmpl, err)
		fmt.Println(msg)
		c.Writer.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Error</title></head><body style='background-color: black; color: white;'><div>%s</div></body></html>`, strings.ReplaceAll(msg, "\n", "<br>"))))
		c.Writer.Flush()
		return
	}
	wasmExecScript, err := script.File(tinygowasmExecLocation).String()
	if err != nil {			log.Fatal("Could not read tinygo wasm_exec.js file: %s\n", err)		}
	htmlPageTemplateData.WasmExecJs  = htmpl.JS(wasmExecScript)
	wasmData, err = script.Exec(`bash -c 'GOOS=js GOARCH=wasm tinygo build -target wasm -o /dev/stdout b.go'`).Bytes()
	if err != nil {
		msg := fmt.Sprintf("Could not compile or read wasm file:\n%s\n%v\n", string(wasmData), err)
		fmt.Println(msg)
		c.Writer.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Error</title></head><body style='background-color: black; color: white;'><div>%s</div></body></html>`, strings.ReplaceAll(msg, "\n", "<br>"))))
		c.Writer.Flush()
		return
	}
	htmlPageTemplateData.WasmBase64  = base64.StdEncoding.EncodeToString(wasmData)
	tmplData := map[string]interface{}{
		"Page":  htmlPageTemplateData,
	}
	var result bytes.Buffer
	err = tmpl.Execute(&result, tmplData)
	if err != nil {
		msg := fmt.Sprintf("Could not execute html template %v\n", err)
		fmt.Println(msg)
		c.Writer.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Error</title></head><body style='background-color: black; color: white;'><div>%s</div></body></html>`, strings.ReplaceAll(msg, "\n", "<br>"))))
		c.Writer.Flush()
		return
	}
	c.Writer.Write(result.Bytes())
	c.Writer.Flush()
})
}






wg.Add(1)
go func() {
fmt.Printf("listening on http://127.0.0.1:%d using gin router\n", webPort)
r1.Run(fmt.Sprintf(":%d", webPort))
wg.Done()
}()
wg.Wait()
},
}

type GinHandler struct { Router *gin.Engine }
func (h *GinHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.Router.ServeHTTP(w, r) }
func loggingMiddleware() gin.HandlerFunc {
return func(c *gin.Context) {
start := time.Now()
c.Next()
latency := time.Since(start)
if latency > time.Minute {			latency = latency.Truncate(time.Second)		}
statusCode := c.Writer.Status()
method := c.Request.Method
path := c.Request.URL.Path
statusCodeBackgroundColor := getBackgroundColor(statusCode)
methodColor := getMethodColor(method)
fmt.Printf("[WASMSTUFF] | %s |%s %3d %s| %13v | %15s | %72s |%s %-7s %s %s\n",			time.Now().Format("2006/01/02 - 15:04:05"),			statusCodeBackgroundColor,			statusCode,			resetColor(),			latency,			c.ClientIP(),			c.Request.RemoteAddr,			methodColor,			method,			resetColor(),			path,		)
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
func resetColor() string {	return reset }
type consoleColorModeValue int
var consoleColorMode = autoColor
const (
	autoColor consoleColorModeValue = iota
	disableColor
	forceColor
)
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
	WasmExecJs  htmpl.JS
	WasmBase64  string
}

const indexHtmpl = `
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>WASM Stuff</title>
<style>
body, html {
margin: 0;
padding: 0;
width: 100%;
height: 100%;
background-color: black;
color: white;
}
#overlay {
padding: 20px;
overflow-y: scroll;
height: 200vh;
position: relative;
z-index: 4;
}
</style>
<script title="wasm_exec.js">
{{.Page.WasmExecJs}}
</script>
<script>
if (!WebAssembly.instantiateStreaming) { // polyfill
  WebAssembly.instantiateStreaming = async (resp, importObject) => {
    const source = await (await resp).arrayBuffer();
    return await WebAssembly.instantiate(source, importObject);
  };
}
const go = new Go();
let mod, inst;

const wasmBase64 = `+"`{{.Page.WasmBase64}}`;"+`
const wasmBinary = Uint8Array.from(atob(wasmBase64), c => c.charCodeAt(0)).buffer;

WebAssembly.instantiate(wasmBinary, go.importObject).then((result) => {
  mod = result.module;
  inst = result.instance;
  run().then((result) => {
    console.log("Ran WASM: ", result)
  }, (failure) => {
    console.log("Failed to run WASM: ", failure)
  })
});
async function run() {
  await go.run(inst);
  inst = await WebAssembly.instantiate(mod, go.importObject); // reset instance
}
</script>
</head>
<body style="margin: 0; padding: 0; width: 100%; height: 100%; background-color: black; color: white;">
<div id='gocanvas-container' style="position: absolute; width: 100%; height: 100%; pointer-events: none; z-index: 3;">
<canvas id='gocanvas' style="max-width: 100%; max-height: 100%; z-index: 3;"></canvas></div>
</body>
</html>
`
