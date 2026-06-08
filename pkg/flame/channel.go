//go:build flame
// +build flame

// Package flame provides Go bindings for the flame RPC (tcs_api).
//
// Each channel is bidirectional: a single channel name maps to one
// request queue + one response queue, mediated by a daemon.
//
// The low-level primitives (Client/Server send/recv) are wrapped by
// rpc.go's RpcClient/RpcServer which adds per-call correlation IDs.
//
// Build requirements:
//   - libflame_rpc.a must exist at /mydata/flame-benchmark/bin/
//   - flame_c_api.h must be at ./flame_rpc/ (vendored copy)

package flame

/*
#cgo CFLAGS:  -I${SRCDIR}
#cgo LDFLAGS: -L/mydata/flame-benchmark/bin -lflame_rpc -lboost_program_options -lrt -lstdc++ -lm

#include "flame_rpc/flame_c_api.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"
)

// Config describes a flame channel.
type Config struct {
	Name       string
	MsgSize    int  // fixed frame size in bytes
	WindowSize int  // outstanding-message buffer depth per side (power-of-2-ish)
	Blocking   bool // true = futex doorbells, false = spin-polling
}

func (c Config) msgSize() C.size_t {
	if c.MsgSize <= 0 {
		return 1024
	}
	return C.size_t(c.MsgSize)
}
func (c Config) window() C.uint32_t {
	if c.WindowSize <= 0 {
		return 256
	}
	return C.uint32_t(c.WindowSize)
}
func (c Config) blocking() C.int {
	if c.Blocking {
		return 1
	}
	return 0
}

// ── Client ───────────────────────────────────────────────────────────────────

// Client is the caller side of a flame channel. Call Send() to push a request,
// then Recv() to read the matching response. Not thread-safe on its own —
// RpcClient (rpc.go) adds the concurrency wrapper.
type Client struct {
	c   *C.FlameClient
	cfg Config
	buf []byte // scratch for Recv
}

// NewClient connects to an existing channel (daemon must be running).
func NewClient(cfg Config) (*Client, error) {
	name := C.CString(cfg.Name)
	defer C.free(unsafe.Pointer(name))

	handle := C.flame_client_connect(name, cfg.msgSize(), cfg.window(), cfg.blocking())
	if handle == nil {
		return nil, fmt.Errorf("flame_client_connect(%q)", cfg.Name)
	}
	cl := &Client{c: handle, cfg: cfg, buf: make([]byte, cfg.MsgSize)}
	runtime.SetFinalizer(cl, (*Client).Close)
	return cl, nil
}

// Send copies buf (≤ MsgSize bytes) into the request queue.
func (c *Client) Send(buf []byte) error {
	if len(buf) == 0 {
		return fmt.Errorf("Send: empty buf")
	}
	if len(buf) > c.cfg.MsgSize {
		return fmt.Errorf("Send: buf too large (%d > %d)", len(buf), c.cfg.MsgSize)
	}
	rc := C.flame_client_send(c.c, unsafe.Pointer(&buf[0]), C.size_t(len(buf)))
	if rc != 0 {
		return fmt.Errorf("flame_client_send")
	}
	return nil
}

// Recv blocks until a response arrives. Returns a Go-owned copy of the frame.
func (c *Client) Recv() ([]byte, error) {
	var outLen C.size_t
	rc := C.flame_client_recv(c.c, unsafe.Pointer(&c.buf[0]),
		C.size_t(len(c.buf)), &outLen)
	if rc != 0 {
		return nil, fmt.Errorf("flame_client_recv")
	}
	out := make([]byte, int(outLen))
	copy(out, c.buf[:outLen])
	return out, nil
}

// SendInPlace acquires a writable shm-backed slot, lets fill() write into
// it, then commits and sends — eliminating the C-side Go-buffer → shm
// memcpy that Send() performs. fill is called with a []byte aliasing the
// shm slot (up to MsgSize bytes) and returns the number of bytes written.
// Not thread-safe on its own; wrap with the same mutex you use for Send.
func (c *Client) SendInPlace(fill func(slot []byte) int) error {
	slotPtr := C.flame_client_alloc_slot(c.c)
	if slotPtr == nil {
		return fmt.Errorf("flame_client_alloc_slot")
	}
	slot := unsafe.Slice((*byte)(slotPtr), c.cfg.MsgSize)
	n := fill(slot)
	if n < 0 || n > c.cfg.MsgSize {
		return fmt.Errorf("SendInPlace: fill returned %d (msg_size=%d)", n, c.cfg.MsgSize)
	}
	rc := C.flame_client_commit_send(c.c, slotPtr, C.size_t(n))
	if rc != 0 {
		return fmt.Errorf("flame_client_commit_send")
	}
	return nil
}

// RecvInPlace blocks until a message arrives, then returns a []byte
// aliasing directly into the shm slot (no copy through a Go scratch
// buffer). The caller MUST invoke release() before the slot can be
// reused; after release() the returned slice is invalid. Not thread-safe.
func (c *Client) RecvInPlace() (slot []byte, release func(), err error) {
	var outLen C.size_t
	slotPtr := C.flame_client_peek_recv(c.c, &outLen)
	if slotPtr == nil {
		return nil, nil, fmt.Errorf("flame_client_peek_recv")
	}
	slot = unsafe.Slice((*byte)(slotPtr), int(outLen))
	release = func() { C.flame_client_release(c.c, slotPtr) }
	return slot, release, nil
}

// Close releases the shm mapping.
func (c *Client) Close() {
	if c.c != nil {
		C.flame_client_destroy(c.c)
		c.c = nil
		runtime.SetFinalizer(c, nil)
	}
}

// ── Server ───────────────────────────────────────────────────────────────────

// Server is the callee side. Call Recv() to read a request, then Send() the
// response. Not thread-safe on its own — RpcServer (rpc.go) serialises.
type Server struct {
	s   *C.FlameServer
	cfg Config
	buf []byte
}

func NewServer(cfg Config) (*Server, error) {
	name := C.CString(cfg.Name)
	defer C.free(unsafe.Pointer(name))

	handle := C.flame_server_connect(name, cfg.msgSize(), cfg.window(), cfg.blocking())
	if handle == nil {
		return nil, fmt.Errorf("flame_server_connect(%q)", cfg.Name)
	}
	sv := &Server{s: handle, cfg: cfg, buf: make([]byte, cfg.MsgSize)}
	runtime.SetFinalizer(sv, (*Server).Close)
	return sv, nil
}

func (s *Server) Recv() ([]byte, error) {
	var outLen C.size_t
	rc := C.flame_server_recv(s.s, unsafe.Pointer(&s.buf[0]),
		C.size_t(len(s.buf)), &outLen)
	if rc != 0 {
		return nil, fmt.Errorf("flame_server_recv")
	}
	out := make([]byte, int(outLen))
	copy(out, s.buf[:outLen])
	return out, nil
}

// SendInPlace: server-side mirror of Client.SendInPlace.
func (s *Server) SendInPlace(fill func(slot []byte) int) error {
	slotPtr := C.flame_server_alloc_slot(s.s)
	if slotPtr == nil {
		return fmt.Errorf("flame_server_alloc_slot")
	}
	slot := unsafe.Slice((*byte)(slotPtr), s.cfg.MsgSize)
	n := fill(slot)
	if n < 0 || n > s.cfg.MsgSize {
		return fmt.Errorf("SendInPlace: fill returned %d (msg_size=%d)", n, s.cfg.MsgSize)
	}
	rc := C.flame_server_commit_send(s.s, slotPtr, C.size_t(n))
	if rc != 0 {
		return fmt.Errorf("flame_server_commit_send")
	}
	return nil
}

// RecvInPlace: server-side mirror of Client.RecvInPlace.
func (s *Server) RecvInPlace() (slot []byte, release func(), err error) {
	var outLen C.size_t
	slotPtr := C.flame_server_peek_recv(s.s, &outLen)
	if slotPtr == nil {
		return nil, nil, fmt.Errorf("flame_server_peek_recv")
	}
	slot = unsafe.Slice((*byte)(slotPtr), int(outLen))
	release = func() { C.flame_server_release(s.s, slotPtr) }
	return slot, release, nil
}

func (s *Server) Send(buf []byte) error {
	if len(buf) == 0 {
		return fmt.Errorf("Send: empty buf")
	}
	if len(buf) > s.cfg.MsgSize {
		return fmt.Errorf("Send: buf too large (%d > %d)", len(buf), s.cfg.MsgSize)
	}
	rc := C.flame_server_send(s.s, unsafe.Pointer(&buf[0]), C.size_t(len(buf)))
	if rc != 0 {
		return fmt.Errorf("flame_server_send")
	}
	return nil
}

func (s *Server) Close() {
	if s.s != nil {
		C.flame_server_destroy(s.s)
		s.s = nil
		runtime.SetFinalizer(s, nil)
	}
}
