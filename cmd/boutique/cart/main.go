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

func addItemToCart(ctx context.Context, req *boutique.AddItemRequest) *boutique.AddItemResponse {
	ok := boutique.AddItem(ctx, req.UserId, req.ProductId, req.Quantity)
	resp := boutique.AddItemResponse{Ok: ok}
	return &resp
}

func getCart(ctx context.Context, req *boutique.GetCartRequest) *boutique.GetCartResponse {
	cart := boutique.GetCart(ctx, req.UserId)
	resp := boutique.GetCartResponse{Cart: cart}
	return &resp
}

func emptyCart(ctx context.Context, req *boutique.EmptyCartRequest) *boutique.EmptyCartResponse {
	ok := boutique.EmptyCart(ctx, req.UserId)
	resp := boutique.EmptyCartResponse{Ok: ok}
	return &resp
}

func addItemToCartFlame(req boutique.AddItemRequest) boutique.AddItemResponse {
	return *addItemToCart(context.Background(), &req)
}

func getCartFlame(req boutique.GetCartRequest) boutique.GetCartResponse {
	return *getCart(context.Background(), &req)
}

func emptyCartFlame(req boutique.EmptyCartRequest) boutique.EmptyCartResponse {
	return *emptyCart(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"add_item":    flame.WrapHandler(addItemToCartFlame),
			"ro_get_cart": flame.WrapHandler(getCartFlame),
			"empty_cart":  flame.WrapHandler(emptyCartFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4101"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/add_item", wrappers.NonROWrapper[boutique.AddItemRequest, boutique.AddItemResponse](addItemToCart))
	http.HandleFunc("/ro_get_cart", wrappers.ROWrapper[boutique.GetCartRequest, boutique.GetCartResponse](getCart))
	http.HandleFunc("/empty_cart", wrappers.NonROWrapper[boutique.EmptyCartRequest, boutique.EmptyCartResponse](emptyCart))
	fmt.Printf("cart listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
