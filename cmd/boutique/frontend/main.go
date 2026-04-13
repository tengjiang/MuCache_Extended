package main

import (
	"context"
	"flag"
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

func home(ctx context.Context, req *boutique.HomeRequest) *boutique.HomeResponse {
	resp := boutique.Home(ctx, *req)
	return &resp
}

//func setCurrency(ctx context.Context, req *boutique.FrontendSetCurrencyRequest) *boutique.FrontendSetCurrencyResponse {
//	boutique.FrontendSetCurrency(ctx, req.Cur)
//	resp := boutique.FrontendSetCurrencyResponse{OK: "OK"}
//	return &resp
//}
func prefetchBrowseProduct(ctx context.Context, req *boutique.BrowseProductRequest) *boutique.BrowseProductResponse {
	resp := boutique.PrefetchBrowseProduct(ctx, req.ProductId)
	return &resp
}

func browseProduct(ctx context.Context, req *boutique.BrowseProductRequest) *boutique.BrowseProductResponse {
	resp := boutique.BrowseProduct(ctx, req.ProductId)
	return &resp
}

func addToCart(ctx context.Context, request *boutique.AddToCartRequest) *boutique.AddToCartResponse {
	resp := boutique.AddToCart(ctx, *request)
	return &resp
}

func viewCart(ctx context.Context, request *boutique.ViewCartRequest) *boutique.ViewCartResponse {
	resp := boutique.ViewCart(ctx, *request)
	return &resp
}

func checkout(ctx context.Context, request *boutique.CheckoutRequest) *boutique.CheckoutResponse {
	resp := boutique.Checkout(ctx, *request)
	return &resp
}

func homeFlame(req boutique.HomeRequest) boutique.HomeResponse {
	return *home(context.Background(), &req)
}

func browseProductFlame(req boutique.BrowseProductRequest) boutique.BrowseProductResponse {
	return *browseProduct(context.Background(), &req)
}

func prefetchBrowseProductFlame(req boutique.BrowseProductRequest) boutique.BrowseProductResponse {
	return *prefetchBrowseProduct(context.Background(), &req)
}

func viewCartFlame(req boutique.ViewCartRequest) boutique.ViewCartResponse {
	return *viewCart(context.Background(), &req)
}

func checkoutFlame(req boutique.CheckoutRequest) boutique.CheckoutResponse {
	return *checkout(context.Background(), &req)
}

func main() {
	prefetch := flag.Bool("prefetch", false, "Flag to enable prefetching")
	flag.Parse()

	fmt.Println("Prefetch flag is ", *prefetch)
	fmt.Println(runtime.GOMAXPROCS(8))

	if common.FLAME {
		browseHandler := browseProductFlame
		if *prefetch {
			browseHandler = prefetchBrowseProductFlame
		}
		flame.StartServer(flame.HandlerRegistry{
			"ro_home":            flame.WrapHandler(homeFlame),
			"ro_browse_product":  flame.WrapHandler(browseHandler),
			"ro_view_cart":       flame.WrapHandler(viewCartFlame),
			"checkout":           flame.WrapHandler(checkoutFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4100"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/ro_home", wrappers.ROWrapper[boutique.HomeRequest, boutique.HomeResponse](home))
	//http.HandleFunc("/set_currency", wrappers.NonROWrapper[boutique.FrontendSetCurrencyRequest, boutique.FrontendSetCurrencyResponse](setCurrency))
	if *prefetch {
		http.HandleFunc("/ro_browse_product", wrappers.ROWrapper[boutique.BrowseProductRequest, boutique.BrowseProductResponse](prefetchBrowseProduct))
	} else {
		http.HandleFunc("/ro_browse_product", wrappers.ROWrapper[boutique.BrowseProductRequest, boutique.BrowseProductResponse](browseProduct))
	}
	//http.HandleFunc("/add_to_cart", wrappers.NonROWrapper[boutique.AddToCartRequest, boutique.AddToCartResponse](addToCart))
	http.HandleFunc("/ro_view_cart", wrappers.ROWrapper[boutique.ViewCartRequest, boutique.ViewCartResponse](viewCart))
	http.HandleFunc("/checkout", wrappers.ROWrapper[boutique.CheckoutRequest, boutique.CheckoutResponse](checkout))
	fmt.Printf("frontend listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
