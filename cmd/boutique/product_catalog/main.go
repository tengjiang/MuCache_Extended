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

func addProduct(ctx context.Context, req *boutique.AddProductRequest) *boutique.AddProductResponse {
	productId := boutique.AddProduct(ctx, req.Product)
	resp := boutique.AddProductResponse{ProductId: productId}
	return &resp
}

func getProduct(ctx context.Context, req *boutique.GetProductRequest) *boutique.GetProductResponse {
	product := boutique.GetProduct(ctx, req.ProductId)
	//fmt.Printf("Product read: %+v\n", product)
	resp := boutique.GetProductResponse{Product: product}
	return &resp
}

func searchProducts(ctx context.Context, req *boutique.SearchProductsRequest) *boutique.SearchProductsResponse {
	products := boutique.SearchProducts(ctx, req.Query)
	//fmt.Printf("Products read: %+v\n", products)
	resp := boutique.SearchProductsResponse{Products: products}
	return &resp
}

func fetchCatalog(ctx context.Context, req *boutique.FetchCatalogRequest) *boutique.FetchCatalogResponse {
	products := boutique.FetchCatalog(ctx, req.CatalogSize)
	resp := boutique.FetchCatalogResponse{Catalog: products}
	return &resp
}

func addProducts(ctx context.Context, req *boutique.AddProductsRequest) *boutique.AddProductsResponse {
	boutique.AddProducts(ctx, req.Products)
	resp := boutique.AddProductsResponse{OK: "OK"}
	return &resp
}

func addProductFlame(req boutique.AddProductRequest) boutique.AddProductResponse {
	return *addProduct(context.Background(), &req)
}

func addProductsFlame(req boutique.AddProductsRequest) boutique.AddProductsResponse {
	return *addProducts(context.Background(), &req)
}

func getProductFlame(req boutique.GetProductRequest) boutique.GetProductResponse {
	return *getProduct(context.Background(), &req)
}

func searchProductsFlame(req boutique.SearchProductsRequest) boutique.SearchProductsResponse {
	return *searchProducts(context.Background(), &req)
}

func fetchCatalogFlame(req boutique.FetchCatalogRequest) boutique.FetchCatalogResponse {
	return *fetchCatalog(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"add_product":        flame.WrapHandler(addProductFlame),
			"add_products":       flame.WrapHandler(addProductsFlame),
			"ro_get_product":     flame.WrapHandler(getProductFlame),
			"ro_search_products": flame.WrapHandler(searchProductsFlame),
			"ro_fetch_catalog":   flame.WrapHandler(fetchCatalogFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4106"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/add_product", wrappers.NonROWrapper[boutique.AddProductRequest, boutique.AddProductResponse](addProduct))
	http.HandleFunc("/add_products", wrappers.NonROWrapper[boutique.AddProductsRequest, boutique.AddProductsResponse](addProducts))
	http.HandleFunc("/ro_get_product", wrappers.ROWrapper[boutique.GetProductRequest, boutique.GetProductResponse](getProduct))
	http.HandleFunc("/ro_search_products", wrappers.ROWrapper[boutique.SearchProductsRequest, boutique.SearchProductsResponse](searchProducts))
	http.HandleFunc("/ro_fetch_catalog", wrappers.ROWrapper[boutique.FetchCatalogRequest, boutique.FetchCatalogResponse](fetchCatalog))
	fmt.Printf("product_catalog listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
