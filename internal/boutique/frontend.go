package boutique

import (
	"context"
	"github.com/DKW2/MuCache_Extended/pkg/invoke"

	"time"
	"fmt"
	"strconv"
	"github.com/google/uuid"
	"github.com/golang/glog"
)

func Home(ctx context.Context, request HomeRequest) HomeResponse {
	req1 := GetSupportedCurrenciesRequest{}
	currenciesRes := invoke.Invoke[GetSupportedCurrenciesResponse](ctx, "currency", "ro_get_currencies", req1)
	//http.HandleFunc("/ro_get_currencies", wrappers.ROWrapper[boutique.GetSupportedCurrenciesRequest, boutique.GetSupportedCurrenciesResponse](getCurrencies))

	req2 := GetCartRequest{UserId: request.Userid}
	cartRes := invoke.Invoke[GetCartResponse](ctx, "cart", "ro_get_cart", req2)
	//http.HandleFunc("/ro_get_cart", wrappers.ROWrapper[boutique.GetCartRequest, boutique.GetCartResponse](getCart))

	req3 := FetchCatalogRequest{CatalogSize: request.CatalogSize}
	catalogRes := invoke.Invoke[FetchCatalogResponse](ctx, "productcatalog", "ro_fetch_catalog", req3)
	//http.HandleFunc("/ro_fetch_catalog", wrappers.ROWrapper[boutique.FetchCatalogRequest, boutique.FetchCatalogResponse](fetchCatalog))

	res := HomeResponse{
		Products:   catalogRes.Catalog,
		UserCart:   cartRes.Cart,
		Currencies: currenciesRes.Currencies,
	}
	return res
}

func FrontendSetCurrency(ctx context.Context, currency Currency) {
	req := SetCurrencySupportRequest{Currency: currency}
	invoke.Invoke[SetCurrencySupportResponse](ctx, "currency", "set_currency", req)
}

func generateCallID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.New().String())
}

func BrowseProduct(ctx context.Context, productId string) BrowseProductResponse {
	req := GetProductRequest{ProductId: productId}
	res := invoke.Invoke[GetProductResponse](ctx, "productcatalog", "ro_get_product", req)
	return BrowseProductResponse{res.Product}
}

func PrefetchBrowseProduct(ctx context.Context, productId string) BrowseProductResponse {
	glog.Infof( "Prefetching product!")
	res := BrowseProduct(ctx, productId)

	// Asynchronous prefetching of adjacent products
	go func() {
		productIDInt, err := strconv.Atoi(productId[1:])
		glog.Infof( "Beginning to prefetch products around product %v", productIDInt )
		glog.Infof( "Any error: %v", err )
		if err == nil {
			prefetchIDs := []string{
				"p" + strconv.Itoa(productIDInt + 1),
				"p" + strconv.Itoa(productIDInt - 1),
			}

			for _, prefetchID := range prefetchIDs {
				// Launch a new goroutine for each prefetch request
				glog.Infof( "Starting to create new goroutine for product %v", prefetchID )
				// prefetchCtx := context.Background()
				// prefetchCtx = context.WithValue(prefetchCtx, "read-only", true)
				// prefetchCtx = context.WithValue(prefetchCtx, "caller", ctx.caller)
				// prefetchCtx = context.WithValue(prefetchCtx, "RID", generateCallID())

				glog.Infof( "Starting goroutine to prefetch product %v", prefetchID )
				go func(prefetchID string) {
					defer func() {
						if r := recover(); r != nil {
							glog.Warningf("Recovered from panic in prefetch for product %v: %v", prefetchID, r)
						}
					}()
					glog.Infof("Prefetching adjacent product: %v", prefetchID)
	
					// Use short-lived background context
					mimicCtx := context.Background()

					for _, key := range []interface{}{"read-only", "caller", "RID", "call-args"} {
						if val := ctx.Value(key); val != nil {
							mimicCtx = context.WithValue(mimicCtx, key, val)
						}
					}

					prefetchCtx, cancel := context.WithTimeout(mimicCtx, 10*time.Millisecond)
					defer cancel()
	
					prefetchReq := GetProductRequest{ProductId: prefetchID}
					invoke.Invoke[GetProductResponse](prefetchCtx, "productcatalog", "ro_get_product", prefetchReq)
				}(prefetchID)
			}
		}
	}()

	return res
}

func AddToCart(ctx context.Context, request AddToCartRequest) AddToCartResponse {
	req := AddItemRequest{
		UserId:    request.UserId,
		ProductId: request.ProductId,
		Quantity:  request.Quantity,
	}
	res := invoke.Invoke[AddItemResponse](ctx, "cart", "add_item", req)
	//http.HandleFunc("/add_item", wrappers.NonROWrapper[boutique.AddItemRequest, boutique.AddItemResponse](addItemToCart))
	return AddToCartResponse{OK: res.Ok}
}

func ViewCart(ctx context.Context, request ViewCartRequest) ViewCartResponse {
	req := GetCartRequest{
		UserId: request.UserId,
	}
	res := invoke.Invoke[GetCartResponse](ctx, "cart", "ro_get_cart", req)
	//http.HandleFunc("/ro_get_cart", wrappers.ROWrapper[boutique.GetCartRequest, boutique.GetCartResponse](getCart))
	return ViewCartResponse{C: res.Cart}
}

func Checkout(ctx context.Context, request CheckoutRequest) CheckoutResponse {
	req := PlaceOrderRequest{
		UserId:       request.UserId,
		UserCurrency: request.UserCurrency,
		Address:      request.Address,
		Email:        request.Email,
		CreditCard:   request.CreditCard,
	}
	res := invoke.Invoke[PlaceOrderResponse](ctx, "checkout", "place_order", req)
	//http.HandleFunc("/place_order", wrappers.NonROWrapper[boutique.PlaceOrderRequest, boutique.PlaceOrderResponse](placeOrder))
	return CheckoutResponse{
		Res: res.Order,
	}
}
