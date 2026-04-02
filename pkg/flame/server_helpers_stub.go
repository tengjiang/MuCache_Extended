//go:build !flame
// +build !flame

package flame

import "encoding/json"

type HandlerRegistry map[string]func([]byte) []byte

func StartServer(handlers HandlerRegistry) {}

// WrapHandler stub — compiled but never called when FLAME=false.
func WrapHandler[Req any, Resp any](handler func(Req) Resp) func([]byte) []byte {
	return func(body []byte) []byte {
		var req Req
		json.Unmarshal(body, &req)
		resp := handler(req)
		out, _ := json.Marshal(resp)
		return out
	}
}
