package main

import (
	"context"
	"fmt"
	"github.com/DKW2/MuCache_Extended/internal/boutique"
	"github.com/DKW2/MuCache_Extended/pkg/common"
	"github.com/DKW2/MuCache_Extended/pkg/flame"
	"github.com/DKW2/MuCache_Extended/pkg/wrappers"
	"net/http"
	"os"
	"runtime"
)

func heartbeat(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("Heartbeat\n"))
	if err != nil {
		return
	}
}

func getRecommendations(ctx context.Context, req *boutique.GetRecommendationsRequest) *boutique.GetRecommendationsResponse {
	products := boutique.GetRecommendations(ctx, req.ProductIds)
	resp := boutique.GetRecommendationsResponse{ProductIds: products}
	return &resp
}

func getRecommendationsFlame(req boutique.GetRecommendationsRequest) boutique.GetRecommendationsResponse {
	return *getRecommendations(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"ro_get_recommendations": flame.WrapHandler(getRecommendationsFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4107"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/ro_get_recommendations", wrappers.ROWrapper[boutique.GetRecommendationsRequest, boutique.GetRecommendationsResponse](getRecommendations))
	fmt.Printf("recommendations listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
