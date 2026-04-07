package flame

import (
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
)

// RpcMsgSize is the fixed frame size for request/response messages.
// Must be large enough for the biggest JSON response (hotel search ~1.2KB).
const RpcMsgSize = 2048

// Layout (1024 bytes):
//   [0:4]     uint32  correlation_id
//   [4:6]     uint16  body_len (actual payload bytes)
//   [6]       uint8   type (1=request, 2=response)
//   [7]       uint8   reserved
//   [8:64]    [56]byte method (null-terminated, requests only)
//   [64:1024] [960]byte body (JSON)

const (
	rpcTypeRequest  = 1
	rpcTypeResponse = 2
	rpcMethodOff    = 8
	rpcMethodLen    = 56
	rpcBodyOff      = 64
	rpcBodyMax      = RpcMsgSize - rpcBodyOff // 960
)

// ── encode / decode ───────────────────────────────────────────────────────────

func rpcEncodeRequest(buf []byte, id uint32, method string, body []byte) {
	for i := range buf { buf[i] = 0 }
	binary.LittleEndian.PutUint32(buf[0:4], id)
	bl := len(body)
	if bl > rpcBodyMax {
		bl = rpcBodyMax
	}
	binary.LittleEndian.PutUint16(buf[4:6], uint16(bl))
	buf[6] = rpcTypeRequest
	ml := len(method)
	if ml > rpcMethodLen-1 {
		ml = rpcMethodLen - 1
	}
	copy(buf[rpcMethodOff:], method[:ml])
	copy(buf[rpcBodyOff:], body[:bl])
}

func rpcEncodeResponse(buf []byte, id uint32, body []byte) {
	for i := range buf { buf[i] = 0 }
	binary.LittleEndian.PutUint32(buf[0:4], id)
	bl := len(body)
	if bl > rpcBodyMax {
		bl = rpcBodyMax
	}
	binary.LittleEndian.PutUint16(buf[4:6], uint16(bl))
	buf[6] = rpcTypeResponse
	copy(buf[rpcBodyOff:], body[:bl])
}

func rpcDecodeID(buf []byte) uint32 {
	return binary.LittleEndian.Uint32(buf[0:4])
}
func rpcDecodeBodyLen(buf []byte) int {
	return int(binary.LittleEndian.Uint16(buf[4:6]))
}
func rpcDecodeType(buf []byte) uint8 {
	return buf[6]
}
func rpcDecodeMethod(buf []byte) string {
	for i := 0; i < rpcMethodLen; i++ {
		if buf[rpcMethodOff+i] == 0 {
			return string(buf[rpcMethodOff : rpcMethodOff+i])
		}
	}
	return string(buf[rpcMethodOff : rpcMethodOff+rpcMethodLen])
}
func rpcDecodeBody(buf []byte) []byte {
	bl := rpcDecodeBodyLen(buf)
	out := make([]byte, bl)
	copy(out, buf[rpcBodyOff:rpcBodyOff+bl])
	return out
}

// ── RpcClient (caller side) ──────────────────────────────────────────────────

// RpcClient sends requests and receives responses over a pair of flame channels.
// Thread-safe for concurrent Call() invocations.
type RpcClient struct {
	reqWriter *Writer
	mu        sync.Mutex             // serialise writes (Writer is not thread-safe)
	pending   sync.Map               // id → chan []byte
	nextID    atomic.Uint32
}

// NewRpcClient connects to an existing channel pair:
//   - writes requests  to  <name>_req  (daemon must have created it)
//   - reads  responses from <name>_resp
func NewRpcClient(name string) (*RpcClient, error) {
	cfg := func(suffix string) Config {
		return Config{Name: name + suffix, MsgSize: RpcMsgSize, Capacity: 256}
	}

	w, err := NewWriter(cfg("_req"))
	if err != nil {
		return nil, fmt.Errorf("RpcClient writer: %w", err)
	}

	c := &RpcClient{reqWriter: w}

	r, err := NewReader(cfg("_resp"), func(msg []byte) {
		id := rpcDecodeID(msg)
		body := rpcDecodeBody(msg)
		if ch, ok := c.pending.Load(id); ok {
			ch.(chan []byte) <- body
		}
	})
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("RpcClient reader: %w", err)
	}

	// Response dispatch loop
	go func() {
		for {
			r.Recv()
		}
	}()

	return c, nil
}

// Call sends a request and blocks until the response arrives.
func (c *RpcClient) Call(method string, body []byte) ([]byte, error) {
	id := c.nextID.Add(1)
	ch := make(chan []byte, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	buf := make([]byte, RpcMsgSize)
	rpcEncodeRequest(buf, id, method, body)

	c.mu.Lock()
	c.reqWriter.Send(buf)
	c.mu.Unlock()

	resp := <-ch
	return resp, nil
}

// ── RpcServer (callee side) ──────────────────────────────────────────────────

// Handler is called for each incoming request. It receives the method name and
// the request body, and must return the response body.
type Handler func(method string, reqBody []byte) []byte

// RpcServer reads requests from <name>_req and writes responses to <name>_resp.
type RpcServer struct {
	respWriter *Writer
	mu         sync.Mutex
}

// NewRpcServer creates a server that dispatches incoming requests to handler.
// The server starts reading immediately in a background goroutine.
func NewRpcServer(name string, handler Handler) (*RpcServer, error) {
	cfg := func(suffix string) Config {
		return Config{Name: name + suffix, MsgSize: RpcMsgSize, Capacity: 256}
	}

	w, err := NewWriter(cfg("_resp"))
	if err != nil {
		return nil, fmt.Errorf("RpcServer resp writer: %w", err)
	}

	s := &RpcServer{respWriter: w}

	r, err := NewReader(cfg("_req"), func(msg []byte) {
		id := rpcDecodeID(msg)
		method := rpcDecodeMethod(msg)
		reqBody := rpcDecodeBody(msg)

		// Dispatch handler in a goroutine so the reader can continue
		go func() {
			respBody := handler(method, reqBody)

			buf := make([]byte, RpcMsgSize)
			rpcEncodeResponse(buf, id, respBody)

			s.mu.Lock()
			s.respWriter.Send(buf)
			s.mu.Unlock()
		}()
	})
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("RpcServer req reader: %w", err)
	}

	// Request receive loop
	go func() {
		for {
			r.Recv()
		}
	}()

	return s, nil
}
