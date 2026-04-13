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

func placeOrder(ctx context.Context, req *boutique.PlaceOrderRequest) *boutique.PlaceOrderResponse {
	order := boutique.PlaceOrder(ctx, req.UserId, req.UserCurrency, req.Address, req.Email, req.CreditCard)
	resp := boutique.PlaceOrderResponse{Order: order}
	return &resp
}

func placeOrderFlame(req boutique.PlaceOrderRequest) boutique.PlaceOrderResponse {
	return *placeOrder(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"place_order": flame.WrapHandler(placeOrderFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4102"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/place_order", wrappers.NonROWrapper[boutique.PlaceOrderRequest, boutique.PlaceOrderResponse](placeOrder))
	fmt.Printf("checkout listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
