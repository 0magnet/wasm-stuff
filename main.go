package main

import (
	"net/http"
	"log"
	"runtime"
	"os"
	"strconv"
	"fmt"
	"sync"
	"strings"
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
func main() {Execute()}

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
var wasmData []byte
var runCmd = &cobra.Command{
Use:   "wasm-stuff",
Short: "wasm attractors",
Long: `
	┬ ┬┌─┐┌─┐┌┬┐   ┌─┐┌┬┐┬ ┬┌─┐┌─┐
	│││├─┤└─┐│││───└─┐ │ │ │├┤ ├┤
	└┴┘┴ ┴└─┘┴ ┴   └─┘ ┴ └─┘└  └  `,
Run: func(_ *cobra.Command, _ []string) {
wg := new(sync.WaitGroup)
wasmExecLocation := runtime.GOROOT() + "/misc/wasm/wasm_exec.js"
wasmExecData, err := script.File(wasmExecLocation).Bytes()
if err != nil {			log.Fatal("Could not read wasm_exec file: %s\n", err)		}
r1 := gin.New()
r1.Use(gin.Recovery())
r1.Use(loggingMiddleware())
r1.GET("/", func(c *gin.Context) {
	c.Writer.Header().Set("Server", "")
	c.Writer.Header().Set("Content-Type", "text/html;charset=utf-8")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()
var indexH []byte
indexH, err = script.File("index.html").Bytes()
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
fmt.Printf("[CUBE] | %s |%s %3d %s| %13v | %15s | %72s |%s %-7s %s %s\n",			time.Now().Format("2006/01/02 - 15:04:05"),			statusCodeBackgroundColor,			statusCode,			resetColor(),			latency,			c.ClientIP(),			c.Request.RemoteAddr,			methodColor,			method,			resetColor(),			path,		)
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
