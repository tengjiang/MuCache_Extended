// hotelpopulate seeds the hotel benchmark with test data via HTTP.
//
// Usage:
//   go run ./cmd/hotelpopulate/ [--frontend=http://localhost:4000] [--user=http://localhost:4005]
//                                [--hotels=20] [--users=50]

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"strings"
)

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
	frontendURL := flag.String("frontend", "http://localhost:4000", "frontend base URL")
	userURL := flag.String("user", "http://localhost:4005", "user service base URL")
	numHotels := flag.Int("hotels", 20, "number of hotels to create")
	numUsers := flag.Int("users", 50, "number of users to register")
	flag.Parse()

	// Register users
	fmt.Printf("Registering %d users...\n", *numUsers)
	for i := 0; i < *numUsers; i++ {
		post(*userURL+"/register_user", map[string]string{
			"username": fmt.Sprintf("user%d", i),
			"password": fmt.Sprintf("pass%d", i),
		})
	}

	// Create hotels (goes through frontend → search, rate, reservation, profile)
	fmt.Printf("Creating %d hotels across 5 cities...\n", *numHotels)
	cities := []string{"city0", "city1", "city2", "city3", "city4"}
	for i := 0; i < *numHotels; i++ {
		city := cities[i%len(cities)]
		post(*frontendURL+"/store_hotel", map[string]interface{}{
			"hotel_id": fmt.Sprintf("hotel_%d", i),
			"name":     fmt.Sprintf("Hotel %d", i),
			"phone":    fmt.Sprintf("555-%04d", i),
			"location": city,
			"rate":     80 + i%50,
			"capacity": 10 + i%5,
			"info":     fmt.Sprintf("Info for hotel %d. %s", i, strings.Repeat("x", 200)),
		})
	}

	fmt.Println("Done! Data seeded.")
	fmt.Println()
	fmt.Println("Test search:")
	fmt.Printf("  curl -s -X POST %s/ro_search_hotels -H 'Content-Type: application/json' \\\n", *frontendURL)
	fmt.Println(`    -d '{"in_date":"2024-01-01","out_date":"2024-01-02","location":"city0"}'`)
}
