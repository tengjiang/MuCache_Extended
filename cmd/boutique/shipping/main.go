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

func getQuote(ctx context.Context, req *boutique.GetQuoteRequest) *boutique.GetQuoteResponse {
	quote := boutique.GetQuote(ctx, req.Items)
	resp := boutique.GetQuoteResponse{CostUsd: quote}
	return &resp
}

func shipOrder(ctx context.Context, req *boutique.ShipOrderRequest) *boutique.ShipOrderResponse {
	id := boutique.ShipOrder(ctx, req.Address, req.Items)
	resp := boutique.ShipOrderResponse{TrackingId: id}
	return &resp
}

func getQuoteFlame(req boutique.GetQuoteRequest) boutique.GetQuoteResponse {
	return *getQuote(context.Background(), &req)
}

func shipOrderFlame(req boutique.ShipOrderRequest) boutique.ShipOrderResponse {
	return *shipOrder(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"ro_get_quote": flame.WrapHandler(getQuoteFlame),
			"ship_order":   flame.WrapHandler(shipOrderFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4108"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/ro_get_quote", wrappers.ROWrapper[boutique.GetQuoteRequest, boutique.GetQuoteResponse](getQuote))
	http.HandleFunc("/ship_order", wrappers.NonROWrapper[boutique.ShipOrderRequest, boutique.ShipOrderResponse](shipOrder))
	fmt.Printf("shipping listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
