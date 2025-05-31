package main

import (
	"context"
	"fmt"
	"github.com/DKW2/MuCache_Extended/internal/movie"
	"github.com/DKW2/MuCache_Extended/pkg/cm"
	"github.com/DKW2/MuCache_Extended/pkg/wrappers"
	"net/http"
	"runtime"
	"flag"
)

func heartbeat(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("Heartbeat\n"))
	if err != nil {
		return
	}
}

func prefetchReadPage(ctx context.Context, req *movie.ReadPageRequest) *movie.ReadPageResponse {
	page := movie.PrefetchReadPage(ctx, req.MovieId)
	//fmt.Printf("Page read: %v\n", page)
	resp := movie.ReadPageResponse{Page: page}
	return &resp
}

func readPage(ctx context.Context, req *movie.ReadPageRequest) *movie.ReadPageResponse {
	page := movie.ReadPage(ctx, req.MovieId)
	//fmt.Printf("Page read: %v\n", page)
	resp := movie.ReadPageResponse{Page: page}
	return &resp
}

func main() {
	prefetch := flag.Bool("prefetch", false, "Flag to enable prefetching")
	flag.Parse()

	fmt.Println( "Prefetch flag is ", *prefetch )

	fmt.Println(runtime.GOMAXPROCS(8))
	for i := 0; i < 1; i++ {  // Adjust worker count based on experiments
		go cm.ZmqProxy()
	}
	http.HandleFunc("/heartbeat", heartbeat)
	if *prefetch {
		http.HandleFunc("/ro_read_page", wrappers.ROWrapper[movie.ReadPageRequest, movie.ReadPageResponse](prefetchReadPage))
	} else {
		http.HandleFunc("/ro_read_page", wrappers.ROWrapper[movie.ReadPageRequest, movie.ReadPageResponse](readPage))
	}
	// http.HandleFunc("/ro_read_page", wrappers.ROWrapper[movie.ReadPageRequest, movie.ReadPageResponse](readPage))
	err := http.ListenAndServe(":3000", nil)
	if err != nil {
		panic(err)
	}
}
