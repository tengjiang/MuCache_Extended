package main

import (
	"flag"
	. "github.com/DKW2/MuCache_Extended/internal/cm"
	"github.com/DKW2/MuCache_Extended/pkg/common"
	_ "net/http/pprof"
)

func main() {
	var Port = flag.Int("port", 80,
		"The port to listen on when in http connection mode.")
	var cmAddressesPath = flag.String("cm_adds", "./experiments/local_cm/twoservices.txt",
		"The file that contains addresses of cache managers of different services.")
	var printTimeFreq = flag.Int("print_time_freq", 1000,
		"Every how many processed events should the cache manager print the time (for throughput measurements).")
	var batchCallInvalidation = flag.Bool("batch_call_invalidation", false,
		"Enable or disable batched invalidation for cache")
	flag.Parse()

	cfg := InitConfig(*Port, *cmAddressesPath, *printTimeFreq, *batchCallInvalidation)
	defer cfg.Close()

	// Initialize the state and call a processor
	state := NewState()
	go Process(cfg, state)
	go HttpSender(cfg)

	if common.ZMQ {
		// HTTP servers are only used between cache managers
		go ServeHttp(cfg)
		Serve0mq(cfg)
	} else {
		ServeHttp(cfg)
	}
}
