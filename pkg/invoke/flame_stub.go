//go:build !flame
// +build !flame

package invoke

func flameInvoke(method string, body []byte) []byte {
	panic("flameInvoke called without flame build tag")
}
