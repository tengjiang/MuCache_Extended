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

func charge(ctx context.Context, req *boutique.ChargeRequest) *boutique.ChargeResponse {
	uid, err := boutique.Charge(ctx, req.Amount, req.CreditCard)
	//fmt.Printf("Products read: %+v\n", products)
	resp := boutique.ChargeResponse{
		Uuid:  uid,
		Error: err,
	}
	return &resp
}

func chargeFlame(req boutique.ChargeRequest) boutique.ChargeResponse {
	return *charge(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"charge": flame.WrapHandler(chargeFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4105"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/charge", wrappers.NonROWrapper[boutique.ChargeRequest, boutique.ChargeResponse](charge))
	fmt.Printf("payment listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
