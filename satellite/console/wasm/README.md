# Using WebAssembly in Storj

In order to use the uplink library from the browser, we can compile the uplink library to WebAssembly (wasm).

### Setup

To generate wasm code that can create access grants in the web browser, run the following from the storj/wasm directory:
```
$ GOOS=js GOARCH=wasm go build -o access.wasm access.go
```

The `access.wasm` code can then be loaded into the browser in a script tag in an html page. Also needed is a JavaScript support file which ships with golang.

To copy the JavaScript support file, run:
```
$ cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" .
```
Ref: [Golang WebAssembly docs](https://github.com/golang/go/wiki/WebAssembly)

The HTML file should include the following:
```
<script type="text/javascript" src="/path/to/wasm_exec.js"></script>
<script>
    const go = new Go();
    WebAssembly.instantiateStreaming(
        fetch("/path/to/access.wasm"), go.importObject).then(
        (result) => {
            go.run(result.instance);
    });
</script>
```

Additionally, the HTTP `Content-Security-Policy (CSP) script-src` directive will need to be modified to allow wasm code to be executed.

See: [WebAssembly Content Security Policy docs](https://github.com/WebAssembly/content-security-policy/blob/master/proposals/CSP.md)
