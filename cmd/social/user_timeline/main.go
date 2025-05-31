package main

import (
	"context"
	"fmt"
	"github.com/DKW2/MuCache_Extended/pkg/cm"
	"github.com/DKW2/MuCache_Extended/pkg/common"
	"github.com/DKW2/MuCache_Extended/pkg/wrappers"
	"net/http"
	"runtime"
	"flag"

	"github.com/DKW2/MuCache_Extended/internal/social"
)

func heartbeat(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("Heartbeat\n"))
	if err != nil {
		return
	}
}

func readUserTimeline(ctx context.Context, req *social.ReadUserTimelineRequest) *social.ReadUserTimelineResponse {
	posts := social.ReadUserTimeline(ctx, req.UserId)
	//fmt.Printf("Posts read: %+v\n", posts)
	resp := social.ReadUserTimelineResponse{Posts: posts}
	return &resp
}

func prefetchSimilarUserTimeline(ctx context.Context, req *social.ReadUserTimelineRequest) *social.ReadUserTimelineResponse {
	posts := social.ReadSimilarUserTimeline(ctx, req.UserId)
	//fmt.Printf("Posts read: %+v\n", posts)
	resp := social.ReadUserTimelineResponse{Posts: posts}
	return &resp
}

func writeUserTimeline(ctx context.Context, req *social.WriteUserTimelineRequest) *string {
	social.WriteUserTimeline(ctx, req.UserId, req.PostIds)
	resp := "OK"
	return &resp
}

func main() {
	prefetch := flag.Bool("prefetch", false, "Flag to enable prefetching")
	flag.Parse()

	fmt.Println( "Prefetch flag is ", *prefetch )

	if common.ShardEnabled {
		fmt.Println(runtime.GOMAXPROCS(1))
	} else {
		fmt.Println(runtime.GOMAXPROCS(8))
	}
	for i := 0; i < 1; i++ {  // Adjust worker count based on experiments
		go cm.ZmqProxy()
	}
	http.HandleFunc("/heartbeat", heartbeat)
	if *prefetch {
		http.HandleFunc("/ro_read_user_timeline", wrappers.ROWrapper[social.ReadUserTimelineRequest, social.ReadUserTimelineResponse](prefetchSimilarUserTimeline))
	} else {
		http.HandleFunc("/ro_read_user_timeline", wrappers.ROWrapper[social.ReadUserTimelineRequest, social.ReadUserTimelineResponse](readUserTimeline))
	}
	http.HandleFunc("/write_user_timeline", wrappers.NonROWrapper[social.WriteUserTimelineRequest, string](writeUserTimeline))
	err := http.ListenAndServe(":3000", nil)
	if err != nil {
		panic(err)
	}
}
