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

func nearby(ctx context.Context, req *hotel.NearbyRequest) *hotel.NearbyResponse {
	rates := hotel.Nearby(ctx, req.InDate, req.OutDate, req.Location)
	//fmt.Printf("Movie info read: %v\n", movieInfo)
	resp := hotel.NearbyResponse{Rates: rates}
	//fmt.Printf("[ReviewStorage] Response: %v\n", resp)
	return &resp
}

func storeHotelLocation(ctx context.Context, req *hotel.StoreHotelLocationRequest) *hotel.StoreHotelLocationResponse {
	hotelId := hotel.StoreHotelLocation(ctx, req.HotelId, req.Location)
	resp := hotel.StoreHotelLocationResponse{HotelId: hotelId}
	//fmt.Printf("[ReviewStorage] Response: %v\n", resp)
	return &resp
}

func nearbyFlame(req hotel.NearbyRequest) hotel.NearbyResponse {
	return *nearby(context.Background(), &req)
}

func storeHotelLocationFlame(req hotel.StoreHotelLocationRequest) hotel.StoreHotelLocationResponse {
	return *storeHotelLocation(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"ro_nearby":            flame.WrapHandler(nearbyFlame),
			"store_hotel_location": flame.WrapHandler(storeHotelLocationFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4001"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/ro_nearby", wrappers.ROWrapper[hotel.NearbyRequest, hotel.NearbyResponse](nearby))
	http.HandleFunc("/store_hotel_location", wrappers.NonROWrapper[hotel.StoreHotelLocationRequest, hotel.StoreHotelLocationResponse](storeHotelLocation))
	fmt.Printf("search listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
