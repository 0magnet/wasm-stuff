# wasm-stuff

Golang WebAssembly (WASM) Example

### Application Server / Development Engine

 The application server, `main.go`, will compile the application and serve it.

 Upon modifying the wasm code in `b.go`, it's only necessary to refresh the page (`F5`) where the application is being served in order to recompile ; saving the step of manual compilation of the wasm binary.

### WASM

The webassembly will spawn it's controls from a minimal html framework in `index0.html`

A selection menu will appear where the model to be rendered may be selected.

Upon a selection, the model will begin to render and controls are provided for zoom and rotation on x, y, and z axis

### Run the application

*Tested on linux

```
git clone https://github.com/0magnet/wasm-stuff
cd wasm-stuff
go run main.go -p 8080
```

access the interface on http://127.0.0.1:8080/

### Serverless / statically

`index.html` is the static implementation where bundle.wasm is included as base64 within the html document itself.

This version is served from http://127.0.0.1:8080/index.html and can be saved / work offline.
