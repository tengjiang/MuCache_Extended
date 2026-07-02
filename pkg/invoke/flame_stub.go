//go:build !flame
// +build !flame

package invoke

func flameInvoke(app string, method string, body []byte) []byte {
	panic("flameInvoke called without flame build tag")
}

func flameInvokeAppend(app, method string, fill func(dst []byte) []byte) []byte {
	panic("flameInvokeAppend called without flame build tag")
}
