// mucache.go — encode/decode MuCache messages as fixed-layout C structs
// and expose a FlameProxy (service→CM writer) and FlameReceiver (CM reader).

package flame

/*
#cgo CFLAGS: -I${SRCDIR}
#include "flame_rpc/mucache_messages.h"
#include <string.h>
#include <stdint.h>

// Helper: safely copy a Go string (ptr+len) into a fixed C char array.
// At most (dst_size - 1) bytes are copied; result is always null-terminated.
static void copy_str(char* dst, const char* src, size_t src_len, size_t dst_size) {
    size_t n = src_len < dst_size - 1 ? src_len : dst_size - 1;
    memcpy(dst, src, n);
    dst[n] = '\0';
}
*/
import "C"
import (
	"encoding/binary"
	"fmt"
	"unsafe"
)

// MsgSize is the recommended flame channel msg_size for MuCache messages.
const MsgSize = int(C.FLAME_MUCACHE_MSG_SIZE)

// ── Encode helpers ─────────────────────────────────────────────────────────

func cstr(dst *C.char, dstSize uintptr, s string) {
	b := []byte(s)
	C.copy_str(dst, (*C.char)(unsafe.Pointer(&b[0])), C.size_t(len(b)), C.size_t(dstSize))
}

// EncodeStart encodes a start event into buf (must be len ≥ MsgSize).
//   - callargs: the CallArgs hash string
//   - appName: the service's app name
func EncodeStart(buf []byte, callargs, appName string) {
	var m C.FlameStartMsg
	m._type = C.FLAME_MSG_START
	cstr(&m.callargs[0], C.FLAME_CALLARGS_MAX, callargs)
	cstr(&m.app_name[0], C.FLAME_APP_NAME_MAX, appName)
	src := (*[C.FLAME_MUCACHE_MSG_SIZE]byte)(unsafe.Pointer(&m))
	copy(buf, src[:C.sizeof_FlameStartMsg])
}

// EncodeEnd encodes an end event into buf (must be len ≥ MsgSize).
func EncodeEnd(buf []byte, callargs, caller string, keyDeps, callDeps []string, retval []byte) {
	var m C.FlameEndMsg
	m._type = C.FLAME_MSG_END

	nk := len(keyDeps)
	if nk > C.FLAME_KEY_DEPS_MAX {
		nk = C.FLAME_KEY_DEPS_MAX
	}
	nc := len(callDeps)
	if nc > C.FLAME_CALL_DEPS_MAX {
		nc = C.FLAME_CALL_DEPS_MAX
	}
	m.n_key_deps = C.uint8_t(nk)
	m.n_call_deps = C.uint8_t(nc)

	cstr(&m.callargs[0], C.FLAME_CALLARGS_MAX, callargs)
	cstr(&m.caller[0], C.FLAME_APP_NAME_MAX, caller)

	for i := 0; i < nk; i++ {
		cstr(&m.key_deps[i][0], C.FLAME_KEY_MAX, keyDeps[i])
	}
	for i := 0; i < nc; i++ {
		cstr(&m.call_deps[i][0], C.FLAME_CALLARGS_MAX, callDeps[i])
	}

	rvlen := len(retval)
	if rvlen > C.FLAME_RETVAL_MAX {
		rvlen = C.FLAME_RETVAL_MAX
	}
	m.retval_len = C.uint32_t(rvlen)
	if rvlen > 0 {
		C.memcpy(unsafe.Pointer(&m.retval[0]), unsafe.Pointer(&retval[0]), C.size_t(rvlen))
	}

	src := (*[C.FLAME_MUCACHE_MSG_SIZE]byte)(unsafe.Pointer(&m))
	copy(buf, src[:C.sizeof_FlameEndMsg])
}

// EncodeInvKey encodes an invalidate-key event into buf.
func EncodeInvKey(buf []byte, key string, fromCM bool) {
	var m C.FlameInvKeyMsg
	m._type = C.FLAME_MSG_INV_KEY
	if fromCM {
		m.from_cm = 1
	}
	cstr(&m.key[0], C.FLAME_KEY_MAX, key)
	src := (*[C.FLAME_MUCACHE_MSG_SIZE]byte)(unsafe.Pointer(&m))
	copy(buf, src[:C.sizeof_FlameInvKeyMsg])
}

// ── Decoded message types ─────────────────────────────────────────────────

type MsgKind uint8

const (
	KindStart  MsgKind = MsgKind(C.FLAME_MSG_START)
	KindEnd    MsgKind = MsgKind(C.FLAME_MSG_END)
	KindInvKey MsgKind = MsgKind(C.FLAME_MSG_INV_KEY)
)

type StartDecoded struct {
	CallArgs string
	AppName  string
}

type EndDecoded struct {
	CallArgs string
	Caller   string
	KeyDeps  []string
	CallDeps []string
	RetVal   []byte
}

type InvKeyDecoded struct {
	Key    string
	FromCM bool
}

// Decode reads the message kind from the first byte of buf.
// Returns one of StartDecoded, EndDecoded, InvKeyDecoded, or an error.
func Decode(buf []byte) (interface{}, error) {
	if len(buf) == 0 {
		return nil, fmt.Errorf("flame.Decode: empty buffer")
	}
	switch MsgKind(buf[0]) {
	case KindStart:
		m := (*C.FlameStartMsg)(unsafe.Pointer(&buf[0]))
		return StartDecoded{
			CallArgs: C.GoString(&m.callargs[0]),
			AppName:  C.GoString(&m.app_name[0]),
		}, nil

	case KindEnd:
		m := (*C.FlameEndMsg)(unsafe.Pointer(&buf[0]))
		nk := int(m.n_key_deps)
		nc := int(m.n_call_deps)
		kd := make([]string, nk)
		for i := range kd {
			kd[i] = C.GoString(&m.key_deps[i][0])
		}
		cd := make([]string, nc)
		for i := range cd {
			cd[i] = C.GoString(&m.call_deps[i][0])
		}
		rvLen := int(binary.LittleEndian.Uint32(
			(*[4]byte)(unsafe.Pointer(&m.retval_len))[:]))
		var rv []byte
		if rvLen > 0 {
			rv = C.GoBytes(unsafe.Pointer(&m.retval[0]), C.int(rvLen))
		}
		return EndDecoded{
			CallArgs: C.GoString(&m.callargs[0]),
			Caller:   C.GoString(&m.caller[0]),
			KeyDeps:  kd,
			CallDeps: cd,
			RetVal:   rv,
		}, nil

	case KindInvKey:
		m := (*C.FlameInvKeyMsg)(unsafe.Pointer(&buf[0]))
		return InvKeyDecoded{
			Key:    C.GoString(&m.key[0]),
			FromCM: m.from_cm != 0,
		}, nil

	default:
		return nil, fmt.Errorf("flame.Decode: unknown message type %d", buf[0])
	}
}
