package main

import (
	"context"
	"fmt"
	"github.com/DKW2/MuCache_Extended/internal/twoservices"
	"github.com/DKW2/MuCache_Extended/pkg/state"
	"github.com/DKW2/MuCache_Extended/pkg/wrappers"
	"github.com/golang/glog"
	"net/http"
	"os"
	"runtime"
	"time"
	//"flag"
	//_ "net/http/pprof"
)

var MaxProcs = 8

func heartbeat(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("Heartbeat\n"))
	if err != nil {
		return
	}
}

func read(ctx context.Context, req *twoserivces.ReadRequest) *twoserivces.ReadResponse {
	//req.K += 1
	start := time.Now() // log arrival timestamp
	v, err := state.GetState[int](ctx, fmt.Sprint(req.K))
	glog.Infof("[backend] GetState took %v", time.Since(start))
	if err != nil {
		v = 0
	}
	resp := twoserivces.ReadResponse{V: v}
	return &resp
}

func write(ctx context.Context, req *twoserivces.WriteRequest) *string {
	state.SetState(ctx, fmt.Sprint(req.K), req.V)
	resp := "OK"
	return &resp
}

func main() {
	// flag.Set("logtostderr", "true")         // Ensure glog logs go to stderr
	// flag.Set("stderrthreshold", "INFO")     // Change to "ERROR" if you want only errors
	// flag.Parse()

	prev := runtime.GOMAXPROCS(MaxProcs)
	fmt.Printf("Set GOMAXPROCS to %d (was %d before)\n", MaxProcs, prev)
	port := os.Getenv("PORT")
	if port == "" {
		port = "3005"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/ro_read", wrappers.ROWrapper[twoserivces.ReadRequest, twoserivces.ReadResponse](read))
	http.HandleFunc("/write", wrappers.NonROWrapper[twoserivces.WriteRequest, string](write))
	fmt.Printf("backend listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
