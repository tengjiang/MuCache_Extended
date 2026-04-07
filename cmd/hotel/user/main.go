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

func registerUser(ctx context.Context, req *hotel.RegisterUserRequest) *hotel.RegisterUserResponse {
	ok := hotel.RegisterUser(ctx, req.Username, req.Password)
	//fmt.Printf("Movie info read: %v\n", movieInfo)
	resp := hotel.RegisterUserResponse{Ok: ok}
	return &resp
}

func login(ctx context.Context, req *hotel.LoginRequest) *hotel.LoginResponse {
	token := hotel.Login(ctx, req.Username, req.Password)
	//fmt.Println("Movie info stored for id: " + movieId)
	resp := hotel.LoginResponse{Token: token}
	return &resp
}

func registerUserFlame(req hotel.RegisterUserRequest) hotel.RegisterUserResponse {
	return *registerUser(context.Background(), &req)
}

func loginFlame(req hotel.LoginRequest) hotel.LoginResponse {
	return *login(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"register_user": flame.WrapHandler(registerUserFlame),
			"login":         flame.WrapHandler(loginFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4005"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/register_user", wrappers.NonROWrapper[hotel.RegisterUserRequest, hotel.RegisterUserResponse](registerUser))
	http.HandleFunc("/login", wrappers.NonROWrapper[hotel.LoginRequest, hotel.LoginResponse](login))
	fmt.Printf("user listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
