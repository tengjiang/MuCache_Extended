// Package flame provides Go bindings for the flame RPC channel (CGO).
//
// It wraps flame_c_api.h (C++) to give Go code a ChannelWriter and
// ChannelReader that communicate via shared memory through the TCS daemon.
//
// Build requirements:
//   - libflame_rpc.a must be compiled and placed at the path set by
//     FLAME_RPC_LIB_DIR (default: /mydata/flame-benchmark/bin).
//   - The flame-benchmark headers must be on the include path
//     (FLAME_RPC_INCLUDE_DIR, default: /mydata/flame-benchmark).

package flame

/*
#cgo CFLAGS:   -I${SRCDIR}
#cgo LDFLAGS:  -L/mydata/flame-benchmark/bin -lflame_rpc -lrt -lstdc++ -lm

#include "flame_rpc/flame_c_api.h"
#include <stdlib.h>

// Forward declaration of the Go-exported callback (void* not const void*,
// matching both the C API typedef and CGO export constraints).
extern void goFlameRecvCb(void* msg, size_t msg_size, void* user_data);
*/
import "C"
import (
	"runtime"
	"sync"
	"unsafe"
)

// ── callback registry ─────────────────────────────────────────────────────────
// CGO cannot pass Go function pointers through C, so we keep a registry of
// Go callbacks indexed by a uintptr handle that is passed as user_data.

type recvFunc func(msg []byte)

var (
	cbMu   sync.Mutex
	cbMap  = make(map[uintptr]recvFunc)
	cbNext uintptr = 1
)

func registerCb(fn recvFunc) uintptr {
	cbMu.Lock()
	defer cbMu.Unlock()
	id := cbNext
	cbNext++
	cbMap[id] = fn
	return id
}

func unregisterCb(id uintptr) {
	cbMu.Lock()
	defer cbMu.Unlock()
	delete(cbMap, id)
}

//export goFlameRecvCb
func goFlameRecvCb(msg unsafe.Pointer, msgSize C.size_t, userData unsafe.Pointer) {
	// userData points to a heap-allocated uint64 holding the callback ID.
	id := uintptr(*(*uint64)(userData))
	cbMu.Lock()
	fn := cbMap[id]
	cbMu.Unlock()
	if fn == nil {
		return
	}
	// Copy into a Go slice (safe; C memory is only valid for this call)
	buf := C.GoBytes(msg, C.int(msgSize))
	fn(buf)
}

// ── Config ────────────────────────────────────────────────────────────────────

// Config mirrors flame::rpc::ChannelConfig.
type Config struct {
	Name     string
	MsgSize  int  // bytes; default 256
	Capacity int  // ring slots; must be power-of-2; default 256
	Doorbell bool // false = polling (default)
}

func (c Config) msgSize() C.size_t {
	if c.MsgSize <= 0 {
		return 256
	}
	return C.size_t(c.MsgSize)
}
func (c Config) capacity() C.size_t {
	if c.Capacity <= 0 {
		return 256
	}
	return C.size_t(c.Capacity)
}
func (c Config) doorbell() C.int {
	if c.Doorbell {
		return 1
	}
	return 0
}

// ── Writer ────────────────────────────────────────────────────────────────────

// Writer sends fixed-size messages to a flame channel via shared memory.
type Writer struct {
	w   *C.FlameWriter
	cfg Config
}

// Connect opens the channel for writing (daemon must be running).
func NewWriter(cfg Config) (*Writer, error) {
	name := C.CString(cfg.Name)
	defer C.free(unsafe.Pointer(name))

	w := C.flame_writer_connect(name, cfg.msgSize(), cfg.capacity(), cfg.doorbell())
	if w == nil {
		return nil, &Error{"flame_writer_connect returned nil"}
	}
	fw := &Writer{w: w, cfg: cfg}
	runtime.SetFinalizer(fw, (*Writer).Close)
	return fw, nil
}

// Send copies buf (must be exactly cfg.MsgSize bytes) into the ring.
// Blocks (spin) if the ring is full.
func (w *Writer) Send(buf []byte) {
	if len(buf) == 0 {
		return
	}
	C.flame_writer_send(w.w, unsafe.Pointer(&buf[0]))
}

// Close unmaps the shared memory.
func (w *Writer) Close() {
	if w.w != nil {
		C.flame_writer_destroy(w.w)
		w.w = nil
		runtime.SetFinalizer(w, nil)
	}
}

// ── Reader ────────────────────────────────────────────────────────────────────

// Reader receives fixed-size messages from a flame channel via shared memory.
type Reader struct {
	r    *C.FlameReader
	cfg  Config
	cbID uintptr
	// idBox holds cbID as a heap-allocated uint64 so we can pass a stable
	// pointer through C without violating Go's unsafe.Pointer rules.
	idBox *uint64
}

// NewReader opens the channel for reading (daemon must be running).
// fn is called for every message received.
func NewReader(cfg Config, fn recvFunc) (*Reader, error) {
	name := C.CString(cfg.Name)
	defer C.free(unsafe.Pointer(name))

	r := C.flame_reader_connect(name, cfg.msgSize(), cfg.capacity(), cfg.doorbell())
	if r == nil {
		return nil, &Error{"flame_reader_connect returned nil"}
	}
	id := registerCb(fn)
	box := new(uint64)
	*box = uint64(id)
	fr := &Reader{r: r, cfg: cfg, cbID: id, idBox: box}
	runtime.SetFinalizer(fr, (*Reader).Close)
	return fr, nil
}

// Recv blocks until the next message is ready, then invokes the callback.
func (r *Reader) Recv() {
	C.flame_reader_recv(r.r,
		(C.FlameRecvCb)(C.goFlameRecvCb),
		unsafe.Pointer(r.idBox))
}

// TryRecv is non-blocking; returns true if a message was delivered.
func (r *Reader) TryRecv() bool {
	return C.flame_reader_try_recv(r.r,
		(C.FlameRecvCb)(C.goFlameRecvCb),
		unsafe.Pointer(r.idBox)) != 0
}

// Close unmaps the shared memory.
func (r *Reader) Close() {
	if r.r != nil {
		C.flame_reader_destroy(r.r)
		r.r = nil
		unregisterCb(r.cbID)
		runtime.SetFinalizer(r, nil)
	}
}

// ── Error ─────────────────────────────────────────────────────────────────────

type Error struct{ msg string }

func (e *Error) Error() string { return "flame: " + e.msg }
