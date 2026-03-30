// Direct Redis implementation replacing the original Dapr-based state API.
// Uses github.com/redis/go-redis/v9 (already a dependency via pkg/cm/cache.go).

package state

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/DKW2/MuCache_Extended/pkg/cm"
	"github.com/DKW2/MuCache_Extended/pkg/common"
	"github.com/DKW2/MuCache_Extended/pkg/wrappers"
	"github.com/goccy/go-json"
	"github.com/golang/glog"
	redis "github.com/redis/go-redis/v9"
)

var stateClient *redis.Client
var stateClientOnce sync.Once

func getStateClient() *redis.Client {
	stateClientOnce.Do(func() {
		stateClient = redis.NewClient(&redis.Options{
			Addr: common.RedisUrl,
		})
	})
	return stateClient
}

func GetState[T interface{}](ctx context.Context, key string) (T, error) {
	if common.CMEnabled {
		wrappers.PreRead(ctx, cm.Key(key))
	}
	rc := getStateClient()
	val, err := rc.Get(ctx, key).Bytes()
	var value T
	if err == redis.Nil {
		glog.Infof("Key Not Found: %v", key)
		return value, errors.New("key not found")
	}
	if err != nil {
		fmt.Printf("Redis GetState error for key %v: %v\n", key, err)
		panic(err)
	}
	err = json.Unmarshal(val, &value)
	if err != nil {
		panic(err)
	}
	return value, nil
}

func GetBulkState[T interface{}](ctx context.Context, keys []string) ([]T, error) {
	if common.CMEnabled {
		for _, key := range keys {
			wrappers.PreRead(ctx, cm.Key(key))
		}
	}
	rc := getStateClient()
	vals, err := rc.MGet(ctx, keys...).Result()
	if err != nil {
		panic(err)
	}
	returnValues := make([]T, len(keys))
	for i, v := range vals {
		if v == nil {
			return nil, errors.New(fmt.Sprintf("key %s not found", keys[i]))
		}
		var value T
		err = json.Unmarshal([]byte(v.(string)), &value)
		if err != nil {
			panic(err)
		}
		returnValues[i] = value
	}
	return returnValues, nil
}

func GetBulkStateDefault[T interface{}](ctx context.Context, keys []string, defVal T) []T {
	if common.CMEnabled {
		for _, key := range keys {
			wrappers.PreRead(ctx, cm.Key(key))
		}
	}
	rc := getStateClient()
	vals, err := rc.MGet(ctx, keys...).Result()
	if err != nil {
		panic(err)
	}
	returnValues := make([]T, len(keys))
	for i, v := range vals {
		if v == nil {
			returnValues[i] = defVal
		} else {
			var value T
			err = json.Unmarshal([]byte(v.(string)), &value)
			if err != nil {
				panic(err)
			}
			returnValues[i] = value
		}
	}
	return returnValues
}

func SetState(ctx context.Context, key string, value interface{}) {
	valueBytes, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	rc := getStateClient()
	err = rc.Set(ctx, key, valueBytes, 0).Err()
	if err != nil {
		panic(err)
	}
	if common.CMEnabled {
		wrappers.PostWrite(ctx, cm.Key(key))
	}
}

func SetBulkState(ctx context.Context, kvs map[string]interface{}) {
	rc := getStateClient()
	pipe := rc.Pipeline()
	for k, v := range kvs {
		valueBytes, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		pipe.Set(ctx, k, valueBytes, 0)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		panic(err)
	}
	if common.CMEnabled {
		for k := range kvs {
			wrappers.PostWrite(ctx, cm.Key(k))
		}
	}
}
