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
// Each handler writes the response body bytes directly into dst (which
// has cap = RpcMsgSize-rpcBodyOff) and returns the extended slice — no
// intermediate Go buffer between the application marshaler and the shm slot.
type HandlerRegistry map[string]func(body []byte, dst []byte) []byte

// StartServer creates flame RPC servers on all upstream channels.
// Reads from:
//   - FLAME_UPSTREAM       — single channel name (chain benchmark)
//   - FLAME_UPSTREAMS      — comma-separated channel names (fan-out benchmarks)
// Does nothing if neither is set (e.g. service1 in chain which receives HTTP).
func StartServer(handlers HandlerRegistry) {
	dispatch := func(method string, reqBody []byte, dst []byte) []byte {
		h, ok := handlers[method]
		if !ok {
			return append(dst, []byte(fmt.Sprintf(`{"error":"unknown method: %s"}`, method))...)
		}
		return h(reqBody, dst)
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

	backend := backendFromEnv()
	for _, ch := range channels {
		_, err := NewRpcServerWithBackend(ch, backend, dispatch)
		if err != nil {
			panic(fmt.Sprintf("flame.StartServer(%q): %v", ch, err))
		}
		fmt.Printf("[flame] server listening on channel %q (backend=%v)\n", ch, backend)
	}
}

// backendFromEnv reads FLAME_BACKEND and maps to a Backend value. Defaults
// to TCS. Duplicates the helper in pkg/invoke/flame.go to keep this
// package free of an import cycle.
func backendFromEnv() Backend {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FLAME_BACKEND"))) {
	case "cq":
		return BackendCQ
	case "cq0":
		return BackendCQ0
	case "", "tcs":
		return BackendTCS
	default:
		panic(fmt.Sprintf("FLAME_BACKEND: unknown value %q", os.Getenv("FLAME_BACKEND")))
	}
}

// binaryAppender is the local mirror of encoding.BinaryAppender (Go 1.24+).
// Types that opt in get the zero-intermediate-buffer fast path on the
// server's response leg.
type binaryAppender interface {
	AppendBinary(dst []byte) ([]byte, error)
}

// WrapHandler creates a handler func from typed Go handler + types.
//
// Encoding preference, in order:
//
//  1. AppendBinary  — writes the response directly into dst (the shm slot
//     body region). No intermediate Go buffer. This is the path the
//     payload-size sweep cares about for large []byte responses.
//  2. MarshalBinary — allocates a fresh []byte, then we append it to dst
//     (still one Go-side allocation per call, but no JSON).
//  3. JSON          — full reflection-based fallback.
//
// Request decode mirrors the choice: prefer BinaryUnmarshaler, fall back
// to JSON. The caller (pkg/invoke.Invoke) makes the symmetric send-side
// choice so the wire formats match.
func WrapHandler[Req any, Resp any](handler func(Req) Resp) func(body []byte, dst []byte) []byte {
	return func(body []byte, dst []byte) []byte {
		var req Req
		if bu, ok := any(&req).(encoding.BinaryUnmarshaler); ok {
			if err := bu.UnmarshalBinary(body); err != nil {
				panic(fmt.Sprintf("flame handler unmarshal: %v", err))
			}
		} else if err := json.Unmarshal(body, &req); err != nil {
			panic(fmt.Sprintf("flame handler unmarshal: %v", err))
		}
		resp := handler(req)
		if ba, ok := any(resp).(binaryAppender); ok {
			out, err := ba.AppendBinary(dst)
			if err != nil {
				panic(fmt.Sprintf("flame handler append: %v", err))
			}
			return out
		}
		if bm, ok := any(resp).(encoding.BinaryMarshaler); ok {
			buf, err := bm.MarshalBinary()
			if err != nil {
				panic(fmt.Sprintf("flame handler marshal: %v", err))
			}
			return append(dst, buf...)
		}
		buf, err := json.Marshal(resp)
		if err != nil {
			panic(fmt.Sprintf("flame handler marshal: %v", err))
		}
		return append(dst, buf...)
	}
}
