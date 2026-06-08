//go:build flame
// +build flame

package flame

import (
	"encoding"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// HandlerRegistry maps method names (e.g. "ro_read") to typed handler functions.
type HandlerRegistry map[string]func([]byte) []byte

// StartServer creates flame RPC servers on all upstream channels.
// Reads from:
//   - FLAME_UPSTREAM       — single channel name (chain benchmark)
//   - FLAME_UPSTREAMS      — comma-separated channel names (fan-out benchmarks)
// Does nothing if neither is set (e.g. service1 in chain which receives HTTP).
func StartServer(handlers HandlerRegistry) {
	dispatch := func(method string, reqBody []byte) []byte {
		h, ok := handlers[method]
		if !ok {
			return []byte(fmt.Sprintf(`{"error":"unknown method: %s"}`, method))
		}
		return h(reqBody)
	}

	var channels []string

	if single := os.Getenv("FLAME_UPSTREAM"); single != "" {
		channels = append(channels, single)
	}

	if multi := os.Getenv("FLAME_UPSTREAMS"); multi != "" {
		for _, ch := range strings.Split(multi, ",") {
			ch = strings.TrimSpace(ch)
			if ch != "" {
				channels = append(channels, ch)
			}
		}
	}

	for _, ch := range channels {
		_, err := NewRpcServer(ch, dispatch)
		if err != nil {
			panic(fmt.Sprintf("flame.StartServer(%q): %v", ch, err))
		}
		fmt.Printf("[flame] server listening on channel %q\n", ch)
	}
}

// WrapHandler creates a handler func from typed Go handler + types.
//
// Prefers per-type binary encoding: if *Req implements
// encoding.BinaryUnmarshaler the body is decoded with UnmarshalBinary;
// if Resp implements encoding.BinaryMarshaler the response is encoded
// with MarshalBinary. Either side independently falls back to JSON.
// The client side (pkg/invoke.Invoke) makes the symmetric choice based
// on the same interface checks, so as long as both sides compile against
// the same type definitions the wire format matches.
func WrapHandler[Req any, Resp any](handler func(Req) Resp) func([]byte) []byte {
	return func(body []byte) []byte {
		var req Req
		if bu, ok := any(&req).(encoding.BinaryUnmarshaler); ok {
			if err := bu.UnmarshalBinary(body); err != nil {
				panic(fmt.Sprintf("flame handler unmarshal: %v", err))
			}
		} else if err := json.Unmarshal(body, &req); err != nil {
			panic(fmt.Sprintf("flame handler unmarshal: %v", err))
		}
		resp := handler(req)
		if bm, ok := any(resp).(encoding.BinaryMarshaler); ok {
			out, err := bm.MarshalBinary()
			if err != nil {
				panic(fmt.Sprintf("flame handler marshal: %v", err))
			}
			return out
		}
		out, err := json.Marshal(resp)
		if err != nil {
			panic(fmt.Sprintf("flame handler marshal: %v", err))
		}
		return out
	}
}
