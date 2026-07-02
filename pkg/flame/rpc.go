//go:build flame
// +build flame

package flame

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DKW2/MuCache_Extended/pkg/latency"
)

// serverRecvGoroutines returns the number of concurrent RecvInPlace loops
// the server should run. Default 1 (single-threaded recv). Setting >1 spawns
// N parallel recv loops sharing the same Server. Whether that's safe depends
// on the C++ backend: TCS and CQ0 use a mutex inside the C++ layer, so
// concurrent peek_recv → release pairs are serialized correctly. CQ uses a
// per-endpoint send-scratch that we already serialize via muSend; recv side
// has no scratch so concurrent peek_recv is fine.
func serverRecvGoroutines() int {
	v := os.Getenv("FLAME_SERVER_RECV_GOROUTINES")
	if v == "" {
		return 1
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// RpcMsgSize is the fixed frame size for request/response messages.
// Must be large enough for the biggest JSON payload in any benchmark.
const RpcMsgSize = 2048

// Layout (2048 bytes):
//   [0:4]     uint32  correlation_id
//   [4:6]     uint16  body_len
//   [6]       uint8   type (1=request, 2=response)
//   [7]       uint8   reserved
//   [8:64]    [56]byte method (null-terminated, requests only)
//   [64:...]  body

const (
	rpcTypeRequest  = 1
	rpcTypeResponse = 2
	rpcMethodOff    = 8
	rpcMethodLen    = 56
	rpcBodyOff      = 64
	rpcBodyMax      = RpcMsgSize - rpcBodyOff
)

// rpcEncodeRequest writes the frame header + method + body into buf.
// Only the bytes that are read by the decoder are overwritten: the 8-byte
// header, the method (with an explicit null terminator), and the body
// region [rpcBodyOff, rpcBodyOff+bl). The unused tail bytes are left
// untouched — the receiver only reads body_len bytes from the header.
func rpcEncodeRequest(buf []byte, id uint32, method string, body []byte) int {
	binary.LittleEndian.PutUint32(buf[0:4], id)
	bl := len(body)
	if bl > rpcBodyMax {
		bl = rpcBodyMax
	}
	binary.LittleEndian.PutUint16(buf[4:6], uint16(bl))
	buf[6] = rpcTypeRequest
	buf[7] = 0 // reserved
	ml := len(method)
	if ml > rpcMethodLen-1 {
		ml = rpcMethodLen - 1
	}
	copy(buf[rpcMethodOff:], method[:ml])
	buf[rpcMethodOff+ml] = 0 // null-terminate method (needed when buf is reused)
	copy(buf[rpcBodyOff:], body[:bl])
	return rpcBodyOff + bl
}

// rpcEncodeResponse: like Request but with no method field. No full-frame
// memset for the same reason — the decoder reads exactly body_len bytes
// of body.
func rpcEncodeResponse(buf []byte, id uint32, body []byte) int {
	binary.LittleEndian.PutUint32(buf[0:4], id)
	bl := len(body)
	if bl > rpcBodyMax {
		bl = rpcBodyMax
	}
	binary.LittleEndian.PutUint16(buf[4:6], uint16(bl))
	buf[6] = rpcTypeResponse
	buf[7] = 0 // reserved
	copy(buf[rpcBodyOff:], body[:bl])
	return rpcBodyOff + bl
}

func rpcDecodeID(buf []byte) uint32   { return binary.LittleEndian.Uint32(buf[0:4]) }
func rpcDecodeBodyLen(buf []byte) int { return int(binary.LittleEndian.Uint16(buf[4:6])) }
func rpcDecodeType(buf []byte) uint8  { return buf[6] }
func rpcDecodeMethod(buf []byte) string {
	for i := 0; i < rpcMethodLen; i++ {
		if buf[rpcMethodOff+i] == 0 {
			return string(buf[rpcMethodOff : rpcMethodOff+i])
		}
	}
	return string(buf[rpcMethodOff : rpcMethodOff+rpcMethodLen])
}
// rpcDecodeBody allocates a fresh []byte for the body. Kept for callers that
// need an independent copy; the hot path uses rpcDecodeBodyView (no copy).
func rpcDecodeBody(buf []byte) []byte {
	bl := rpcDecodeBodyLen(buf)
	out := make([]byte, bl)
	copy(out, buf[rpcBodyOff:rpcBodyOff+bl])
	return out
}

// rpcDecodeBodyView returns a slice aliasing the body bytes inside buf.
// The caller must not retain the slice past the lifetime of buf — in the
// flame fast path that's fine because the handler (server) or
// json.Unmarshal (client) consumes the body synchronously.
func rpcDecodeBodyView(buf []byte) []byte {
	bl := rpcDecodeBodyLen(buf)
	return buf[rpcBodyOff : rpcBodyOff+bl]
}

// ── RpcClient ────────────────────────────────────────────────────────────────

// RpcClient wraps a Client with correlation-id based request/response
// matching, so multiple goroutines can call Call() concurrently.
type RpcClient struct {
	cl      *Client
	muSend  sync.Mutex // serialise Send (single-writer queue)
	pending sync.Map   // id → chan []byte
	nextID  atomic.Uint32
}

// RpcWindowSize is the per-side buffer count. Must satisfy
// `window_size <= 2 * queue_capacity` (the TCS assertion). flame-benchmark's
// `shared_memory_queue_capacity_shift` is bumped from 8 → 11 on this branch,
// giving queue_capacity = 2048 and a window_size ceiling of 4096.
//
// We set window_size = 2 * queue_capacity so the client's request-buffer
// cursor has a full queue worth of margin before wrapping. Bigger window =
// more in-flight RPCs per hop before flame.Send() backpressures, which is
// what keeps the open-loop curve from cliff-collapsing at high rates.
const RpcWindowSize = 4096

// NewRpcClient connects to an existing bidirectional channel using the
// default backend (TCS). For other backends call NewRpcClientWithBackend.
func NewRpcClient(name string) (*RpcClient, error) {
	return NewRpcClientWithBackend(name, BackendTCS)
}

// NewRpcClientWithBackend connects with an explicit backend choice
// (must match the daemon's --backend).
func NewRpcClientWithBackend(name string, backend Backend) (*RpcClient, error) {
	cfg := Config{Name: name, Backend: backend, MsgSize: RpcMsgSize, WindowSize: RpcWindowSize, Blocking: true}
	cl, err := NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("RpcClient: %w", err)
	}

	c := &RpcClient{cl: cl}

	// Response dispatch goroutine — reads each response in place from the
	// shm slot, copies just the body_len bytes the caller needs, then
	// releases the slot. The body_len copy is much smaller than the old
	// msg_size scratch+fresh-[]byte copy pair.
	go func() {
		for {
			slot, release, err := cl.RecvInPlace()
			if err != nil {
				return
			}
			if len(slot) < rpcBodyOff {
				release()
				continue
			}
			id := rpcDecodeID(slot)
			view := rpcDecodeBodyView(slot) // aliases shm
			body := make([]byte, len(view)) // small body-sized copy
			copy(body, view)
			release()
			if ch, ok := c.pending.Load(id); ok {
				select {
				case ch.(chan []byte) <- body:
				default:
				}
			}
		}
	}()

	return c, nil
}

// Call sends a request and blocks until the matching response arrives.
// Safe for concurrent use by multiple goroutines. The request frame is
// encoded directly into a shm slot — no Go-side send buffer, no C-side
// memcpy from Go to shm.
func (c *RpcClient) Call(method string, body []byte) ([]byte, error) {
	id := c.nextID.Add(1)
	ch := make(chan []byte, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	t0 := time.Now()
	c.muSend.Lock()
	latency.Record("rpc_send_lock_wait", time.Since(t0))
	t1 := time.Now()
	err := c.cl.SendInPlace(func(slot []byte) int {
		return rpcEncodeRequest(slot, id, method, body)
	})
	c.muSend.Unlock()
	latency.Record("rpc_send", time.Since(t1))
	if err != nil {
		return nil, err
	}

	t2 := time.Now()
	resp := <-ch
	latency.Record("rpc_wait_response", time.Since(t2))
	return resp, nil
}

func (c *RpcClient) Close() { c.cl.Close() }

// CallAppend is the zero-intermediate-buffer variant of Call.
// The fill callback writes the request body directly into dst (which is
// the body region of the shm slot, len=0 cap=rpcBodyMax). Returns the
// response body bytes (the dispatch goroutine has copied them out of
// shm into a fresh []byte).
func (c *RpcClient) CallAppend(method string, fill func(dst []byte) []byte) ([]byte, error) {
	id := c.nextID.Add(1)
	ch := make(chan []byte, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	t0 := time.Now()
	c.muSend.Lock()
	latency.Record("rpc_send_lock_wait", time.Since(t0))
	t1 := time.Now()
	err := c.cl.SendInPlace(func(slot []byte) int {
		bodyDst := slot[rpcBodyOff:rpcBodyOff] // len=0, cap=msg_size-rpcBodyOff
		out := fill(bodyDst)
		n := len(out)
		// Write frame header in place after we know n.
		binary.LittleEndian.PutUint32(slot[0:4], id)
		binary.LittleEndian.PutUint16(slot[4:6], uint16(n))
		slot[6] = rpcTypeRequest
		slot[7] = 0
		ml := len(method)
		if ml > rpcMethodLen-1 {
			ml = rpcMethodLen - 1
		}
		copy(slot[rpcMethodOff:], method[:ml])
		slot[rpcMethodOff+ml] = 0
		return rpcBodyOff + n
	})
	c.muSend.Unlock()
	latency.Record("rpc_send", time.Since(t1))
	if err != nil {
		return nil, err
	}

	t2 := time.Now()
	resp := <-ch
	latency.Record("rpc_wait_response", time.Since(t2))
	return resp, nil
}

// ── RpcServer ────────────────────────────────────────────────────────────────

// Handler appends the response body bytes into dst (len=0, cap=rpcBodyMax)
// and returns the extended slice. body holds the request bytes. The
// caller does NOT pre-encode a body []byte and copy it into shm — the
// handler writes directly into the shm slot via dst, eliminating the
// intermediate Go buffer that the old Handler signature required.
type Handler func(method string, reqBody []byte, dst []byte) []byte

// RpcServer reads requests from the channel, dispatches to handler, sends
// responses back. Supports concurrent request handling via per-request
// goroutines; responses are serialised through a single Send mutex.
type RpcServer struct {
	sv     *Server
	muSend sync.Mutex
}

// NewRpcServer opens the channel using the default backend (TCS) and
// starts a goroutine that reads requests and dispatches to handler.
// Each request handler runs in its own goroutine.
func NewRpcServer(name string, handler Handler) (*RpcServer, error) {
	return NewRpcServerWithBackend(name, BackendTCS, handler)
}

// NewRpcServerWithBackend opens the channel with an explicit backend.
func NewRpcServerWithBackend(name string, backend Backend, handler Handler) (*RpcServer, error) {
	cfg := Config{Name: name, Backend: backend, MsgSize: RpcMsgSize, WindowSize: RpcWindowSize, Blocking: true}
	sv, err := NewServer(cfg)
	if err != nil {
		return nil, fmt.Errorf("RpcServer: %w", err)
	}
	s := &RpcServer{sv: sv}
	// CQ (with copies) returns the SAME endpoint-owned scratch pointer
	// from every alloc_slot — concurrent SendInPlace callers would race.
	// TCS and CQ0 hand out distinct slots per call (TCS pool, CQ0 ring
	// internal_buffers) so they're concurrency-safe via the C++ mutex.
	needsSendLock := backend == BackendCQ

	recvLoop := func() {
		for {
			slot, release, err := sv.RecvInPlace()
			if err != nil {
				return
			}
			if len(slot) < rpcBodyOff {
				release()
				continue
			}
			id := rpcDecodeID(slot)
			method := rpcDecodeMethod(slot)
			view := rpcDecodeBodyView(slot) // aliases shm
			body := make([]byte, len(view)) // small body-sized copy so the slot can be released
			copy(body, view)
			release()

			go func() {
				t0 := time.Now()
				t1 := time.Now()
				if needsSendLock {
					// CQ path: handler runs OUTSIDE the lock (concurrent),
					// result gets copied into the shm slot under the lock.
					// One body-sized memcpy + an alloc per RPC, but
					// handlers parallelize properly.
					respBuf := make([]byte, 0, RpcMsgSize-rpcBodyOff)
					out := handler(method, body, respBuf)
					n := len(out)
					s.muSend.Lock()
					s.sv.SendInPlace(func(slot []byte) int {
						binary.LittleEndian.PutUint32(slot[0:4], id)
						binary.LittleEndian.PutUint16(slot[4:6], uint16(n))
						slot[6] = rpcTypeResponse
						slot[7] = 0
						copy(slot[rpcBodyOff:], out)
						return rpcBodyOff + n
					})
					s.muSend.Unlock()
				} else {
					// TCS / CQ0: alloc returns distinct slots per call,
					// so handler can run inside SendInPlace's callback
					// (writes directly into shm — zero intermediate
					// buffer).
					s.sv.SendInPlace(func(slot []byte) int {
						bodyDst := slot[rpcBodyOff:rpcBodyOff] // len=0, cap=msg_size-rpcBodyOff
						out := handler(method, body, bodyDst)
						n := len(out)
						binary.LittleEndian.PutUint32(slot[0:4], id)
						binary.LittleEndian.PutUint16(slot[4:6], uint16(n))
						slot[6] = rpcTypeResponse
						slot[7] = 0
						return rpcBodyOff + n
					})
				}
				latency.Record("rpc_server_send", time.Since(t1))
				latency.Record("rpc_server_handler", time.Since(t0))
			}()
		}
	}

	n := serverRecvGoroutines()
	for i := 0; i < n; i++ {
		go recvLoop()
	}

	return s, nil
}

func (s *RpcServer) Close() { s.sv.Close() }
