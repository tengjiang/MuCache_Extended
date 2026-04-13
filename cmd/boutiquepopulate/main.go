// boutiquepopulate seeds the boutique benchmark via the frontend/checkout services.
//
// Usage:
//   go run ./cmd/boutiquepopulate/ [--frontend=http://localhost:4100] ...

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
)

type Money struct {
	Currency string `json:"currencyCode"`
	Units    int32  `json:"units"`
	Nanos    int64  `json:"nanos"`
}

type Product struct {
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Picture     string   `json:"picture"`
	PriceUsd    *Money   `json:"priceUSD"`
	Categories  []string `json:"categories"`
}

type Currency struct {
	CurrencyCode string `json:"currencyCode"`
	Rate         string `json:"rate"`
}

type AddProductsRequest struct {
	Products []Product `json:"products"`
}

type InitCurrencyRequest struct {
	Currencies []Currency `json:"currencies"`
}

func post(url string, body interface{}) {
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		panic(fmt.Sprintf("POST %s: %v", url, err))
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		panic(fmt.Sprintf("POST %s: status %d", url, resp.StatusCode))
	}
}

func main() {
	currencyURL := flag.String("currency", "http://localhost:4103", "currency service URL")
	pcURL := flag.String("productcatalog", "http://localhost:4106", "product catalog URL")
	numProducts := flag.Int("products", 50, "number of products to create")
	flag.Parse()

	// Seed a few common currencies (rates relative to EUR).
	currencies := []Currency{
		{CurrencyCode: "EUR", Rate: "1.0"},
		{CurrencyCode: "USD", Rate: "1.1305"},
		{CurrencyCode: "JPY", Rate: "126.40"},
		{CurrencyCode: "GBP", Rate: "0.85"},
		{CurrencyCode: "CAD", Rate: "1.51"},
	}
	fmt.Printf("Registering %d currencies...\n", len(currencies))
	post(*currencyURL+"/init_currencies", InitCurrencyRequest{Currencies: currencies})

	// Seed products (priced in USD).
	fmt.Printf("Creating %d products...\n", *numProducts)
	products := make([]Product, *numProducts)
	for i := 0; i < *numProducts; i++ {
		products[i] = Product{
			Id:          fmt.Sprintf("p%d", i),
			Name:        fmt.Sprintf("Product %d", i),
			Description: fmt.Sprintf("Description of product %d", i),
			Picture:     fmt.Sprintf("/img/p%d.jpg", i),
			PriceUsd: &Money{
				Currency: "USD",
				Units:    int32(10 + i%90),
				Nanos:    0,
			},
			Categories: []string{"category" + fmt.Sprint(i%5)},
		}
	}
	post(*pcURL+"/add_products", AddProductsRequest{Products: products})

	fmt.Println("Done! Boutique seeded.")
	fmt.Println()
	fmt.Println("Test home:")
	fmt.Println(`  curl -s -X POST http://localhost:4100/ro_home -H 'Content-Type: application/json' \`)
	fmt.Println(`    -d '{"user_id":"user_0","catalog_size":10}'`)
}
