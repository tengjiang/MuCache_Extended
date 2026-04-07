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

func searchHotels(ctx context.Context, req *hotel.SearchHotelsRequest) *hotel.SearchHotelsResponse {
	hotels := hotel.SearchHotels(ctx, req.InDate, req.OutDate, req.Location)
	//fmt.Printf("Movie info read: %v\n", movieInfo)
	resp := hotel.SearchHotelsResponse{Profiles: hotels}
	//fmt.Printf("[ReviewStorage] Response: %v\n", resp)
	return &resp
}

func storeHotel(ctx context.Context, req *hotel.StoreHotelRequest) *hotel.StoreHotelResponse {
	hotelId := hotel.StoreHotel(ctx, req.HotelId, req.Name, req.Phone, req.Location, req.Rate, req.Capacity, req.Info)
	//fmt.Printf("Movie info read: %v\n", movieInfo)
	resp := hotel.StoreHotelResponse{HotelId: hotelId}
	//fmt.Printf("[ReviewStorage] Response: %v\n", resp)
	return &resp
}

func reservation(ctx context.Context, req *hotel.FrontendReservationRequest) *hotel.FrontendReservationResponse {
	success := hotel.FrontendReservation(ctx, req.HotelId, req.InDate, req.OutDate, req.Rooms, req.Username, req.Password)
	resp := hotel.FrontendReservationResponse{Success: success}
	return &resp
}

func searchHotelsFlame(req hotel.SearchHotelsRequest) hotel.SearchHotelsResponse {
	return *searchHotels(context.Background(), &req)
}

func storeHotelFlame(req hotel.StoreHotelRequest) hotel.StoreHotelResponse {
	return *storeHotel(context.Background(), &req)
}

func reservationFlame(req hotel.FrontendReservationRequest) hotel.FrontendReservationResponse {
	return *reservation(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"ro_search_hotels": flame.WrapHandler(searchHotelsFlame),
			"store_hotel":      flame.WrapHandler(storeHotelFlame),
			"reservation":      flame.WrapHandler(reservationFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/ro_search_hotels", wrappers.ROWrapper[hotel.SearchHotelsRequest, hotel.SearchHotelsResponse](searchHotels))
	http.HandleFunc("/store_hotel", wrappers.NonROWrapper[hotel.StoreHotelRequest, hotel.StoreHotelResponse](storeHotel))
	http.HandleFunc("/reservation", wrappers.NonROWrapper[hotel.FrontendReservationRequest, hotel.FrontendReservationResponse](reservation))
	fmt.Printf("frontend listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
