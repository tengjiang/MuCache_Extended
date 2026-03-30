//go:build !k8s
// +build !k8s

package nodeIdx

import "os"

// In local mode, NODE_IDX env var specifies which line in the CM address file
// this cache manager instance is responsible for (1-indexed).
var NodeIdx = os.Getenv("NODE_IDX")
