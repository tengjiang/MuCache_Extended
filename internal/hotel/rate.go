package hotel

import (
	"context"
	"github.com/DKW2/MuCache_Extended/pkg/state"
)

const ratePrefix = "rate:"

func StoreRate(ctx context.Context, rate Rate) string {
	state.SetState(ctx, ratePrefix+rate.HotelId, rate)
	return rate.HotelId
}

func GetRates(ctx context.Context, hotelIds []string) []Rate {
	rates := make([]Rate, len(hotelIds))
	for i, hotelId := range hotelIds {
		rate, err := state.GetState[Rate](ctx, ratePrefix+hotelId)
		if err != nil {
			panic(err)
		}
		rates[i] = rate
	}
	//fmt.Printf("[ReviewStorage] Returning: %v\n", reviews)
	return rates
}
