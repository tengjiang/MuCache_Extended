package boutique

import (
	"context"
	"github.com/DKW2/MuCache_Extended/pkg/state"
	"strings"
)

var CatalogSize = 1000

const productPrefix = "product:"
const productKeysListKey = "product:KEYS"

func AddProduct(ctx context.Context, product Product) string {
	keys, err := state.GetState[[]string](ctx, productKeysListKey)
	if err != nil {
		// empty db
	}
	keys = append(keys, product.Id)
	state.SetState(ctx, productKeysListKey, keys)
	state.SetState(ctx, productPrefix+product.Id, product)
	return product.Id
}

func AddProducts(ctx context.Context, products []Product) {
	keys, err := state.GetState[[]string](ctx, productKeysListKey)
	if err != nil {
		// empty db
	}
	if len(keys) < CatalogSize {
		rest := CatalogSize - len(keys)
		if len(products) < rest {
			rest = len(products)
		}
		for i := 0; i < rest; i++ {
			keys = append(keys, products[i].Id)
		}
		state.SetState(ctx, productKeysListKey, keys)
	}

	productMap := make(map[string]interface{})
	for _, product := range products {
		productMap[productPrefix+product.Id] = product
	}
	state.SetBulkState(ctx, productMap)
	return
}

func GetProduct(ctx context.Context, Id string) Product {
	product, err := state.GetState[Product](ctx, productPrefix+Id)
	if err != nil {
		panic(err)
	}
	return product
}

func SearchProducts(ctx context.Context, name string) []Product {
	products := make([]Product, 0)
	keys, err := state.GetState[[]string](ctx, productKeysListKey)
	if err != nil {
		panic(err)
	}
	for _, id := range keys {
		product, err := state.GetState[Product](ctx, productPrefix+id)
		if err != nil {
			panic(err)
		}
		if strings.Contains(strings.ToLower(product.Name), strings.ToLower(name)) {
			products = append(products, product)
		}
	}
	return products
}

func FetchCatalog(ctx context.Context, catalogSize int) []Product {
	keys, err := state.GetState[[]string](ctx, productKeysListKey)
	if err != nil {
		panic(err)
	}

	if catalogSize < len(keys) {
		keys = keys[:catalogSize]
	}
	var products []Product
	if len(keys) > 0 {
		prefixed := make([]string, len(keys))
		for i, k := range keys {
			prefixed[i] = productPrefix + k
		}
		products = state.GetBulkStateDefault[Product](ctx, prefixed, Product{})
	} else {
		products = make([]Product, len(keys))
	}
	return products
}
