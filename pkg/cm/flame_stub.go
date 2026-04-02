//go:build !flame
// +build !flame

package cm

// StartFlame is a no-op when the flame build tag is not set.
func StartFlame() {}
