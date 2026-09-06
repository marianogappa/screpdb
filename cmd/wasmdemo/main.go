//go:build js && wasm

package main

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"syscall/js"

	"github.com/marianogappa/screpdb/internal/dashboard"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/sampledata"
)

const appRoot = "/screpdb"

func main() {
	if err := os.Setenv("SCREPDB_APPDATA_DIR", appRoot); err != nil {
		js.Global().Get("console").Call("error", fmt.Sprintf("setenv: %v", err))
		return
	}

	iofacade.AllowDir(appRoot)
	for _, dir := range []string{appRoot, appRoot + "/sample_replays"} {
		if err := iofacade.MkdirAll(dir, 0o755); err != nil {
			js.Global().Get("console").Call("error", fmt.Sprintf("mkdir %s: %v", dir, err))
			return
		}
	}

	if err := sampledata.Extract(appRoot + "/sample_replays"); err != nil {
		js.Global().Get("console").Call("error", fmt.Sprintf("extract samples: %v", err))
		return
	}

	ctx := context.Background()
	dash, err := dashboard.New(ctx, dashboard.Options{
		Root:      appRoot,
		ReplayDir: appRoot + "/sample_replays",
	})
	if err != nil {
		js.Global().Get("console").Call("error", fmt.Sprintf("dashboard.New: %v", err))
		return
	}
	defer dash.Close()

	handler := dash.Handler()

	js.Global().Set("__screpdb_handleRequest", js.FuncOf(func(_ js.Value, args []js.Value) any {
		method := args[0].String()
		url := args[1].String()
		body := args[2].String()
		callback := args[3]

		go func() {
			req := httptest.NewRequest(method, url, strings.NewReader(body))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			result := w.Result()
			defer result.Body.Close()
			respBody, _ := io.ReadAll(result.Body)

			headers := js.Global().Get("Object").New()
			for key, vals := range result.Header {
				headers.Set(key, strings.Join(vals, ", "))
			}

			response := js.Global().Get("Object").New()
			response.Set("status", result.StatusCode)
			response.Set("headers", headers)

			uint8Array := js.Global().Get("Uint8Array").New(len(respBody))
			js.CopyBytesToJS(uint8Array, respBody)
			response.Set("body", uint8Array)

			callback.Invoke(response)
		}()

		return nil
	}))

	// Expose a subscribe function that JS calls when the FakeLibrarySocket
	// connects. This ensures the initial snapshot is delivered to the right
	// callback instead of being dropped during WASM init.
	js.Global().Set("__screpdb_subscribeLibraryEvents", js.FuncOf(func(_ js.Value, args []js.Value) any {
		callback := args[0]
		dash.OnLibraryEvent(func(data []byte) {
			callback.Invoke(string(data))
		})
		return nil
	}))

	if err := dash.StartLibrary(); err != nil {
		js.Global().Get("console").Call("error", fmt.Sprintf("StartLibrary: %v", err))
	}

	js.Global().Set("__screpdb_ready", true)
	js.Global().Get("console").Call("log", "screpdb WASM backend ready")

	select {}
}
