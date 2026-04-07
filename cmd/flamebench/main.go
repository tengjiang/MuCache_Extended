//go:build flame

// flamebench — all-shm benchmark client for the chain benchmark.
//
// Sends requests directly to service1 via flame shm (no HTTP entry point).
// Measures end-to-end latency through the full 5-hop chain.
//
// Prerequisites:
//   bash scripts/local/start_chain.sh flame   # starts daemons + services
//
// Then run:
//   FLAME_DOWNSTREAM=hop0 go run -tags flame ./cmd/flamebench/ -n 5000 -c 50
//
// The start script must also start a daemon pair for "hop0" (client→service1)
// and service1 must have FLAME_UPSTREAMS=hop0 (or FLAME_UPSTREAM=hop0).

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DKW2/MuCache_Extended/pkg/flame"
)

type ReadRequest struct {
	K int `json:"k"`
}

type ReadResponse struct {
	V int `json:"v"`
}

func main() {
	n := flag.Int("n", 5000, "total number of requests")
	c := flag.Int("c", 50, "concurrency (parallel workers)")
	warmupN := flag.Int("warmup", 500, "warmup requests (not measured)")
	flag.Parse()

	channelName := os.Getenv("FLAME_DOWNSTREAM")
	if channelName == "" {
		fmt.Fprintln(os.Stderr, "error: set FLAME_DOWNSTREAM=hop0 (the client→service1 channel)")
		os.Exit(1)
	}

	client, err := flame.NewRpcClient(channelName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: RpcClient(%q): %v\n", channelName, err)
		os.Exit(1)
	}

	reqBody, _ := json.Marshal(ReadRequest{K: 1})

	// warmup
	fmt.Printf("Warming up (%d requests)...\n", *warmupN)
	for i := 0; i < *warmupN; i++ {
		resp, err := client.Call("ro_read", reqBody)
		if err != nil {
			panic(err)
		}
		if i == 0 {
			var r ReadResponse
			json.Unmarshal(resp, &r)
			fmt.Printf("  smoke test: {v: %d}\n", r.V)
		}
	}

	// benchmark
	fmt.Printf("Benchmarking: %d requests, %d workers...\n", *n, *c)
	latencies := make([]time.Duration, *n)
	var idx atomic.Int64
	var wg sync.WaitGroup

	start := time.Now()

	for w := 0; w < *c; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(idx.Add(1) - 1)
				if i >= *n {
					return
				}
				t0 := time.Now()
				_, err := client.Call("ro_read", reqBody)
				latencies[i] = time.Since(t0)
				if err != nil {
					panic(err)
				}
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	// stats
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}

	pct := func(p float64) time.Duration { return latencies[int(math.Floor(float64(*n)*p))] }

	rps := float64(*n) / elapsed.Seconds()

	fmt.Println()
	fmt.Println("Results:")
	fmt.Printf("  Total:        %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  Requests/sec: %.0f\n", rps)
	fmt.Printf("  Avg:          %v\n", sum/time.Duration(*n))
	fmt.Printf("  Min:          %v\n", latencies[0])
	fmt.Printf("  p50:          %v\n", pct(0.50))
	fmt.Printf("  p90:          %v\n", pct(0.90))
	fmt.Printf("  p99:          %v\n", pct(0.99))
	fmt.Printf("  Max:          %v\n", latencies[*n-1])
}
