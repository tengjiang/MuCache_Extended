//go:build !k8s
// +build !k8s

package common

import "os"

// In local setup, urls are read from environment variables at startup.
// Set before running each service binary:
//   CM_URL=http://localhost:9001   (the cache manager for this specific service)
//   REDIS_URL=localhost:6379       (Redis address, used by both CM and backend state)

var MemcachedUrl = getEnvOrDefault("REDIS_URL", "localhost:6379")
var CachedUrl = getEnvOrDefault("REDIS_URL", "localhost:6379")
var RedisUrl = getEnvOrDefault("REDIS_URL", "localhost:6379")
var CMUrl = getEnvOrDefault("CM_URL", "http://localhost:9001")

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
