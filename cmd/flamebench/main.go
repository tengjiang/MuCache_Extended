//go:build flame

// flamebench: direct latency of flame shm vs HTTP wrapper→CM transport.
// Start the daemon first:
//   /mydata/flame-benchmark/bin/flame_daemon --channel-name bench --msg-size 1280 --capacity 256
// Then:
//   go run -tags flame ./cmd/flamebench/

package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/DKW2/MuCache_Extended/pkg/flame"
)

const (
	N      = 50_000
	warmup = 5_000
)

func pct(s []time.Duration, p float64) time.Duration {
	return s[int(float64(len(s))*p/100)]
}

// ── flame round-trip ──────────────────────────────────────────────────────────

func benchFlame() []time.Duration {
	cfg := flame.Config{Name: "bench", MsgSize: flame.MsgSize, Capacity: 256}

	w, err := flame.NewWriter(cfg)
	if err != nil {
		panic(err)
	}

	recv := make(chan struct{}, 1)
	r, err := flame.NewReader(cfg, func(_ []byte) {
		select {
		case recv <- struct{}{}:
		default:
		}
	})
	if err != nil {
		panic(err)
	}

	// Keep recv goroutine running; stop it via a done channel
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				r.TryRecv() // non-blocking; tight-loop until done
			}
		}
	}()

	buf := make([]byte, flame.MsgSize)
	flame.EncodeStart(buf, "deadbeef", "bench_svc")

	for i := 0; i < warmup; i++ {
		w.Send(buf)
		<-recv
	}

	lats := make([]time.Duration, N)
	for i := 0; i < N; i++ {
		t0 := time.Now()
		w.Send(buf)
		<-recv
		lats[i] = time.Since(t0)
	}

	close(done)
	w.Close()
	r.Close()
	return lats
}

// ── HTTP round-trip (loopback POST to CM /start) ──────────────────────────────

func benchHTTP(cmURL string) []time.Duration {
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 64,
			MaxConnsPerHost:     64,
		},
	}
	body := []byte(`{"callargs":"deadbeef"}`)

	// warmup
	for i := 0; i < warmup; i++ {
		resp, err := client.Post(cmURL+"/start", "application/json", bytes.NewReader(body))
		if err != nil {
			panic(err)
		}
		resp.Body.Close()
	}

	lats := make([]time.Duration, N)
	for i := 0; i < N; i++ {
		t0 := time.Now()
		resp, err := client.Post(cmURL+"/start", "application/json", bytes.NewReader(body))
		if err != nil {
			panic(fmt.Sprintf("HTTP POST failed: %v", err))
		}
		resp.Body.Close()
		lats[i] = time.Since(t0)
	}
	return lats
}

// ── main ──────────────────────────────────────────────────────────────────────

func report(name string, lats []time.Duration) {
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	var sum time.Duration
	for _, l := range lats {
		sum += l
	}
	fmt.Printf("%-20s  min=%6v  avg=%6v  p50=%6v  p90=%6v  p99=%6v  p99.9=%7v  max=%6v\n",
		name,
		lats[0],
		sum/time.Duration(len(lats)),
		pct(lats, 50),
		pct(lats, 90),
		pct(lats, 99),
		pct(lats, 99.9),
		lats[len(lats)-1],
	)
}

func main() {
	const cmURL = "http://localhost:9001"

	fmt.Printf("Measuring transport latency (N=%d each, send+receive ack)\n\n", N)

	fmt.Printf("Running HTTP benchmark (→ %s)...\n", cmURL)
	httpLats := benchHTTP(cmURL)

	fmt.Println("Running flame shm benchmark (→ bench channel)...")
	flameLats := benchFlame()

	fmt.Println()
	fmt.Printf("%-20s  %6s   %6s   %6s   %6s   %6s   %7s   %6s\n",
		"transport", "min", "avg", "p50", "p90", "p99", "p99.9", "max")
	fmt.Println(string(bytes.Repeat([]byte("-"), 90)))
	report("HTTP (loopback)", httpLats)
	report("Flame shm (TCS)", flameLats)
	fmt.Println()

	// speedup
	httpAvg := func() time.Duration {
		var s time.Duration
		for _, l := range httpLats {
			s += l
		}
		return s / time.Duration(len(httpLats))
	}()
	flameAvg := func() time.Duration {
		var s time.Duration
		for _, l := range flameLats {
			s += l
		}
		return s / time.Duration(len(flameLats))
	}()
	fmt.Printf("Flame avg speedup vs HTTP: %.1fx\n", float64(httpAvg)/float64(flameAvg))
}
