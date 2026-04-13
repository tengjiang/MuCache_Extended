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

func sendEmail(ctx context.Context, req *boutique.SendOrderConfirmationRequest) *boutique.SendOrderConfirmationResponse {
	ok := boutique.SendConfirmation(ctx, req.Email, req.Order)
	resp := boutique.SendOrderConfirmationResponse{Ok: ok}
	return &resp
}

func sendEmailFlame(req boutique.SendOrderConfirmationRequest) boutique.SendOrderConfirmationResponse {
	return *sendEmail(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"ro_send_email": flame.WrapHandler(sendEmailFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4104"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/ro_send_email", wrappers.ROWrapper[boutique.SendOrderConfirmationRequest, boutique.SendOrderConfirmationResponse](sendEmail))
	fmt.Printf("email listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
