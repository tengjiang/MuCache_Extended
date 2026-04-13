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

func setCurrency(ctx context.Context, req *boutique.SetCurrencySupportRequest) *boutique.SetCurrencySupportResponse {
	ok := boutique.SetCurrencySupport(ctx, req.Currency)
	resp := boutique.SetCurrencySupportResponse{Ok: ok}
	return &resp
}

func getCurrencies(ctx context.Context, req *boutique.GetSupportedCurrenciesRequest) *boutique.GetSupportedCurrenciesResponse {
	currencies := boutique.GetSupportedCurrencies(ctx)
	resp := boutique.GetSupportedCurrenciesResponse{Currencies: currencies}
	return &resp
}

func convertCurrency(ctx context.Context, req *boutique.ConvertCurrencyRequest) *boutique.ConvertCurrencyResponse {
	amount := boutique.ConvertCurrency(ctx, req.Amount, req.ToCurrency)
	resp := boutique.ConvertCurrencyResponse{Amount: amount}
	return &resp
}

func initCurrencies(ctx context.Context, req *boutique.InitCurrencyRequest) *boutique.InitCurrencyResponse {
	boutique.InitCurrencies(ctx, req.Currencies)
	resp := boutique.InitCurrencyResponse{Ok: "OK"}
	return &resp
}

func setCurrencyFlame(req boutique.SetCurrencySupportRequest) boutique.SetCurrencySupportResponse {
	return *setCurrency(context.Background(), &req)
}

func getCurrenciesFlame(req boutique.GetSupportedCurrenciesRequest) boutique.GetSupportedCurrenciesResponse {
	return *getCurrencies(context.Background(), &req)
}

func convertCurrencyFlame(req boutique.ConvertCurrencyRequest) boutique.ConvertCurrencyResponse {
	return *convertCurrency(context.Background(), &req)
}

func initCurrenciesFlame(req boutique.InitCurrencyRequest) boutique.InitCurrencyResponse {
	return *initCurrencies(context.Background(), &req)
}

func main() {
	fmt.Println(runtime.GOMAXPROCS(8))
	if common.FLAME {
		flame.StartServer(flame.HandlerRegistry{
			"set_currency":        flame.WrapHandler(setCurrencyFlame),
			"init_currencies":     flame.WrapHandler(initCurrenciesFlame),
			"ro_get_currencies":   flame.WrapHandler(getCurrenciesFlame),
			"ro_convert_currency": flame.WrapHandler(convertCurrencyFlame),
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4103"
	}
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/set_currency", wrappers.NonROWrapper[boutique.SetCurrencySupportRequest, boutique.SetCurrencySupportResponse](setCurrency))
	http.HandleFunc("/init_currencies", wrappers.NonROWrapper[boutique.InitCurrencyRequest, boutique.InitCurrencyResponse](initCurrencies))
	http.HandleFunc("/ro_get_currencies", wrappers.ROWrapper[boutique.GetSupportedCurrenciesRequest, boutique.GetSupportedCurrenciesResponse](getCurrencies))
	http.HandleFunc("/ro_convert_currency", wrappers.ROWrapper[boutique.ConvertCurrencyRequest, boutique.ConvertCurrencyResponse](convertCurrency))
	fmt.Printf("currency listening on :%s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
