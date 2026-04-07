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

func storeProfile(ctx context.Context, req *hotel.StoreProfileRequest) *hotel.StoreProfileResponse {
	hotelId := hotel.StoreProfile(ctx, req.Profile)
	//fmt.Println("Movie info stored for id: " + movieId)
	resp := hotel.StoreProfileResponse{HotelId: hotelId}
	return &resp
}

func getProfiles(ctx context.Context, req *hotel.GetProfilesRequest) *hotel.GetProfilesResponse {
	hotels := hotel.GetProfiles(ctx, req.HotelIds)
	//fmt.Printf("Movie info read: %v\n", movieInfo)
	resp := hotel.GetProfilesResponse{Profiles: hotels}
	//fmt.Printf("[ReviewStorage] Response: %v\n", resp)
	return &resp
}

func getProfilesFlame(req hotel.GetProfilesRequest) hotel.GetProfilesResponse {
	return *getProfiles(context.Background(), &req)
}

func storeProfileFlame(req hotel.StoreProfileRequest) hotel.StoreProfileResponse {
	return *storeProfile(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"ro_get_profiles": flame.WrapHandler(getProfilesFlame),
			"store_profile":   flame.WrapHandler(storeProfileFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4003"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/store_profile", wrappers.NonROWrapper[hotel.StoreProfileRequest, hotel.StoreProfileResponse](storeProfile))
	http.HandleFunc("/ro_get_profiles", wrappers.ROWrapper[hotel.GetProfilesRequest, hotel.GetProfilesResponse](getProfiles))
	fmt.Printf("profile listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
