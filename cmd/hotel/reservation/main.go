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

func checkAvailability(ctx context.Context, req *hotel.CheckAvailabilityRequest) *hotel.CheckAvailabilityResponse {
	hotelIds := hotel.CheckAvailability(ctx, req.CustomerName, req.HotelIds, req.InDate, req.OutDate, req.RoomNumber)
	//fmt.Println("Movie info stored for id: " + movieId)
	resp := hotel.CheckAvailabilityResponse{HotelIds: hotelIds}
	return &resp
}

func makeReservation(ctx context.Context, req *hotel.MakeReservationRequest) *hotel.MakeReservationResponse {
	success := hotel.MakeReservation(ctx, req.CustomerName, req.HotelId, req.InDate, req.OutDate, req.RoomNumber)
	//fmt.Printf("Movie info read: %v\n", movieInfo)
	resp := hotel.MakeReservationResponse{Success: success}
	//fmt.Printf("[ReviewStorage] Response: %v\n", resp)
	return &resp
}

func addHotelAvailability(ctx context.Context, req *hotel.AddHotelAvailabilityRequest) *hotel.AddHotelAvailabilityResponse {
	hotelId := hotel.AddHotelAvailability(ctx, req.HotelId, req.Capacity)
	resp := hotel.AddHotelAvailabilityResponse{Hotelid: hotelId}
	return &resp
}

func checkAvailabilityFlame(req hotel.CheckAvailabilityRequest) hotel.CheckAvailabilityResponse {
	return *checkAvailability(context.Background(), &req)
}

func makeReservationFlame(req hotel.MakeReservationRequest) hotel.MakeReservationResponse {
	return *makeReservation(context.Background(), &req)
}

func addHotelAvailabilityFlame(req hotel.AddHotelAvailabilityRequest) hotel.AddHotelAvailabilityResponse {
	return *addHotelAvailability(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"ro_check_availability":  flame.WrapHandler(checkAvailabilityFlame),
			"make_reservation":       flame.WrapHandler(makeReservationFlame),
			"add_hotel_availability": flame.WrapHandler(addHotelAvailabilityFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4004"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	// Note: Even though checkAvailability is ReadOnly, the developers could explicitly decide to not have it be cached,
	//       because that could lead to stale results being seen by users
	//       (though not really since the invalidation should take <1s and this is less time
	//        than what a person needs until they look at a list of results anyway).
	http.HandleFunc("/ro_check_availability", wrappers.ROWrapper[hotel.CheckAvailabilityRequest, hotel.CheckAvailabilityResponse](checkAvailability))
	http.HandleFunc("/make_reservation", wrappers.NonROWrapper[hotel.MakeReservationRequest, hotel.MakeReservationResponse](makeReservation))
	http.HandleFunc("/add_hotel_availability", wrappers.NonROWrapper[hotel.AddHotelAvailabilityRequest, hotel.AddHotelAvailabilityResponse](addHotelAvailability))
	fmt.Printf("reservation listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
