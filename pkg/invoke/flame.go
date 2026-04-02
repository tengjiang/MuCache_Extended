//go:build flame
// +build flame

package invoke

import (
	"fmt"
	"os"
	"sync"

	"github.com/DKW2/MuCache_Extended/pkg/flame"
)

// flameClient is the global RPC client for the downstream service.
// Created lazily on first call.
var (
	flameClient     *flame.RpcClient
	flameClientOnce sync.Once
)

func getFlameClient() *flame.RpcClient {
	flameClientOnce.Do(func() {
		name := os.Getenv("FLAME_DOWNSTREAM")
		if name == "" {
			return // no downstream (e.g., backend)
		}
		var err error
		flameClient, err = flame.NewRpcClient(name)
		if err != nil {
			panic(fmt.Sprintf("flame RpcClient(%q): %v", name, err))
		}
	})
	return flameClient
}

// flameInvoke sends the request over shared memory and returns the raw response bytes.
func flameInvoke(method string, body []byte) []byte {
	c := getFlameClient()
	if c == nil {
		panic("flameInvoke: no FLAME_DOWNSTREAM configured")
	}
	resp, err := c.Call(method, body)
	if err != nil {
		panic(fmt.Sprintf("flameInvoke: %v", err))
	}
	return resp
}
