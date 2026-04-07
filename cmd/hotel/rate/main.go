package main

import (
	"context"
	"fmt"
	"github.com/DKW2/MuCache_Extended/internal/hotel"
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

func storeRate(ctx context.Context, req *hotel.StoreRateRequest) *hotel.StoreRateResponse {
	hotelId := hotel.StoreRate(ctx, req.Rate)
	//fmt.Println("Movie info stored for id: " + movieId)
	resp := hotel.StoreRateResponse{HotelId: hotelId}
	return &resp
}

func getRates(ctx context.Context, req *hotel.GetRatesRequest) *hotel.GetRatesResponse {
	rates := hotel.GetRates(ctx, req.HotelIds)
	//fmt.Printf("Movie info read: %v\n", movieInfo)
	resp := hotel.GetRatesResponse{Rates: rates}
	//fmt.Printf("[ReviewStorage] Response: %v\n", resp)
	return &resp
}

func getRatesFlame(req hotel.GetRatesRequest) hotel.GetRatesResponse {
	return *getRates(context.Background(), &req)
}

func storeRateFlame(req hotel.StoreRateRequest) hotel.StoreRateResponse {
	return *storeRate(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"ro_get_rates": flame.WrapHandler(getRatesFlame),
			"store_rate":   flame.WrapHandler(storeRateFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4002"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/store_rate", wrappers.NonROWrapper[hotel.StoreRateRequest, hotel.StoreRateResponse](storeRate))
	http.HandleFunc("/ro_get_rates", wrappers.ROWrapper[hotel.GetRatesRequest, hotel.GetRatesResponse](getRates))
	fmt.Printf("rate listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
