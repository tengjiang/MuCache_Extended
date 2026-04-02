//go:build !flame
// +build !flame

package cm

// ServeFlame is a no-op when the flame build tag is not set.
func ServeFlame(cfg *Config) {}
