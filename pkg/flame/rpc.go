//go:build flame
// +build flame

package flame

import (
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DKW2/MuCache_Extended/pkg/latency"
)

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

// responseBufPool reuses RpcMsgSize-sized scratch buffers for encoding
// server responses, eliminating an allocation per RPC. Each handler
// goroutine Gets a buffer, encodes, Sends (which copies into shm), then
// Puts the buffer back.
var responseBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, RpcMsgSize)
		return &b
	},
}

// ── RpcClient ────────────────────────────────────────────────────────────────

// RpcClient wraps a Client with correlation-id based request/response
// matching, so multiple goroutines can call Call() concurrently.
type RpcClient struct {
	cl      *Client
	muSend  sync.Mutex // serialise Send (single-writer queue)
	pending sync.Map   // id → chan []byte
	nextID  atomic.Uint32
	sendBuf []byte
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

// NewRpcClient connects to an existing bidirectional channel.
func NewRpcClient(name string) (*RpcClient, error) {
	cfg := Config{Name: name, MsgSize: RpcMsgSize, WindowSize: RpcWindowSize, Blocking: true}
	cl, err := NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("RpcClient: %w", err)
	}

	c := &RpcClient{
		cl:      cl,
		sendBuf: make([]byte, RpcMsgSize),
	}

	// Response dispatch goroutine — reads responses and routes by id.
	go func() {
		for {
			msg, err := cl.Recv()
			if err != nil {
				return
			}
			if len(msg) < rpcBodyOff {
				continue
			}
			id := rpcDecodeID(msg)
			body := rpcDecodeBodyView(msg) // alias into msg; consumed before next Recv
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
// Safe for concurrent use by multiple goroutines.
func (c *RpcClient) Call(method string, body []byte) ([]byte, error) {
	id := c.nextID.Add(1)
	ch := make(chan []byte, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	t0 := time.Now()
	c.muSend.Lock()
	latency.Record("rpc_send_lock_wait", time.Since(t0))
	t1 := time.Now()
	n := rpcEncodeRequest(c.sendBuf, id, method, body)
	err := c.cl.Send(c.sendBuf[:n])
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

// ── RpcServer ────────────────────────────────────────────────────────────────

// Handler takes the method name + request body, returns the response body.
type Handler func(method string, reqBody []byte) []byte

// RpcServer reads requests from the channel, dispatches to handler, sends
// responses back. Supports concurrent request handling via per-request
// goroutines; responses are serialised through a single Send mutex.
type RpcServer struct {
	sv     *Server
	muSend sync.Mutex
}

// NewRpcServer opens the channel and starts a goroutine that reads requests
// and dispatches to handler. Each request handler runs in its own goroutine.
func NewRpcServer(name string, handler Handler) (*RpcServer, error) {
	cfg := Config{Name: name, MsgSize: RpcMsgSize, WindowSize: RpcWindowSize, Blocking: true}
	sv, err := NewServer(cfg)
	if err != nil {
		return nil, fmt.Errorf("RpcServer: %w", err)
	}
	s := &RpcServer{sv: sv}

	go func() {
		for {
			msg, err := sv.Recv()
			if err != nil {
				return
			}
			if len(msg) < rpcBodyOff {
				continue
			}
			id := rpcDecodeID(msg)
			method := rpcDecodeMethod(msg)
			body := rpcDecodeBodyView(msg) // alias into msg; handler unmarshals synchronously

			go func() {
				t0 := time.Now()
				respBody := handler(method, body)
				latency.Record("rpc_server_handler", time.Since(t0))

				// Reuse a pooled 2048-byte scratch instead of allocating per response.
				bufp := responseBufPool.Get().(*[]byte)
				buf := *bufp
				n := rpcEncodeResponse(buf, id, respBody)

				t1 := time.Now()
				s.muSend.Lock()
				s.sv.Send(buf[:n])
				s.muSend.Unlock()
				latency.Record("rpc_server_send", time.Since(t1))

				// Send copied buf into shm; safe to return to pool now.
				responseBufPool.Put(bufp)
			}()
		}
	}()

	return s, nil
}

func (s *RpcServer) Close() { s.sv.Close() }
