//go:build flame
// +build flame

package flame

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// HandlerRegistry maps method names (e.g. "ro_read") to typed handler functions.
// Each handler receives the raw JSON request body and returns the raw JSON response body.
type HandlerRegistry map[string]func([]byte) []byte

// StartServer creates and starts a flame RPC server on the upstream channel.
// Does nothing if FLAME_UPSTREAM is empty (e.g. service1 which receives HTTP).
func StartServer(handlers HandlerRegistry) {
	name := os.Getenv("FLAME_UPSTREAM")
	if name == "" {
		return
	}

	_, err := NewRpcServer(name, func(method string, reqBody []byte) []byte {
		h, ok := handlers[method]
		if !ok {
			errMsg := fmt.Sprintf(`{"error":"unknown method: %s"}`, method)
			return []byte(errMsg)
		}
		return h(reqBody)
	})
	if err != nil {
		panic(fmt.Sprintf("flame.StartServer(%q): %v", name, err))
	}
	fmt.Printf("[flame] server listening on channel %q\n", name)
}

// WrapHandler creates a handler func from typed Go handler + types.
// This avoids duplicating json marshal/unmarshal logic in every service.
func WrapHandler[Req any, Resp any](handler func(Req) Resp) func([]byte) []byte {
	return func(body []byte) []byte {
		var req Req
		if err := json.Unmarshal(body, &req); err != nil {
			panic(fmt.Sprintf("flame handler unmarshal: %v", err))
		}
		resp := handler(req)
		out, err := json.Marshal(resp)
		if err != nil {
			panic(fmt.Sprintf("flame handler marshal: %v", err))
		}
		return out
	}
}

// WaitForever blocks the calling goroutine. Use after StartServer to keep main alive.
var waitForeverOnce sync.Once

func WaitForever() {
	select {}
}
