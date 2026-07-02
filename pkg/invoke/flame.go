//go:build flame
// +build flame

package invoke

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/DKW2/MuCache_Extended/pkg/flame"
	"github.com/DKW2/MuCache_Extended/pkg/latency"
)

// flameClients maps service name → RpcClient.
// Built lazily from FLAME_CHANNELS_FILE (format: "appname channelname" per line).
var (
	flameClients     map[string]*flame.RpcClient
	flameClientsOnce sync.Once
)

func getFlameClients() map[string]*flame.RpcClient {
	flameClientsOnce.Do(func() {
		flameClients = make(map[string]*flame.RpcClient)
		backend := parseFlameBackendEnv()

		// Legacy single-downstream (chain benchmark)
		if single := os.Getenv("FLAME_DOWNSTREAM"); single != "" {
			callee := os.Getenv("FLAME_DOWNSTREAM_APP")
			if callee == "" {
				callee = "_default"
			}
			c, err := flame.NewRpcClientWithBackend(single, backend)
			if err != nil {
				panic(fmt.Sprintf("flame RpcClient(%q): %v", single, err))
			}
			flameClients[callee] = c
		}

		// Multi-downstream (hotel, social)
		path := os.Getenv("FLAME_CHANNELS_FILE")
		if path == "" {
			return
		}
		f, err := os.Open(path)
		if err != nil {
			panic(fmt.Sprintf("FLAME_CHANNELS_FILE: %v", err))
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				continue
			}
			app := strings.TrimSpace(parts[0])
			channel := strings.TrimSpace(parts[1])
			if _, exists := flameClients[app]; exists {
				continue // already registered (e.g. from FLAME_DOWNSTREAM)
			}
			c, err := flame.NewRpcClientWithBackend(channel, backend)
			if err != nil {
				panic(fmt.Sprintf("flame RpcClient(%q→%q): %v", app, channel, err))
			}
			flameClients[app] = c
		}
	})
	return flameClients
}

// parseFlameBackendEnv reads FLAME_BACKEND ∈ {tcs, cq, cq0} (case-insensitive).
// Defaults to TCS. Exported via a helper so server_helpers.go shares the same parsing.
func parseFlameBackendEnv() flame.Backend {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FLAME_BACKEND"))) {
	case "cq":
		return flame.BackendCQ
	case "cq0":
		return flame.BackendCQ0
	case "", "tcs":
		return flame.BackendTCS
	default:
		panic(fmt.Sprintf("FLAME_BACKEND: unknown value %q (want tcs|cq|cq0)", os.Getenv("FLAME_BACKEND")))
	}
}

// flameInvoke sends the request over shared memory and returns the raw response bytes.
func flameInvoke(app string, method string, body []byte) []byte {
	clients := getFlameClients()
	c := clients[app]
	if c == nil {
		c = clients["_default"]
	}
	if c == nil {
		panic(fmt.Sprintf("flameInvoke: no flame channel for app %q", app))
	}
	t0 := time.Now()
	resp, err := c.Call(method, body)
	latency.Record("flame_rpc_call", time.Since(t0))
	if err != nil {
		panic(fmt.Sprintf("flameInvoke(%s/%s): %v", app, method, err))
	}
	return resp
}

// flameInvokeAppend uses RpcClient.CallAppend: the fill callback writes
// the request body directly into the shm slot's body region. No Go-side
// intermediate buffer between the application marshaler and the slot.
func flameInvokeAppend(app, method string, fill func(dst []byte) []byte) []byte {
	clients := getFlameClients()
	c := clients[app]
	if c == nil {
		c = clients["_default"]
	}
	if c == nil {
		panic(fmt.Sprintf("flameInvokeAppend: no flame channel for app %q", app))
	}
	t0 := time.Now()
	resp, err := c.CallAppend(method, fill)
	latency.Record("flame_rpc_call", time.Since(t0))
	if err != nil {
		panic(fmt.Sprintf("flameInvokeAppend(%s/%s): %v", app, method, err))
	}
	return resp
}
