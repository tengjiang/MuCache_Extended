package flame

import (
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
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

func rpcEncodeRequest(buf []byte, id uint32, method string, body []byte) int {
	for i := range buf {
		buf[i] = 0
	}
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
	return rpcBodyOff + bl
}

func rpcEncodeResponse(buf []byte, id uint32, body []byte) int {
	for i := range buf {
		buf[i] = 0
	}
	binary.LittleEndian.PutUint32(buf[0:4], id)
	bl := len(body)
	if bl > rpcBodyMax {
		bl = rpcBodyMax
	}
	binary.LittleEndian.PutUint16(buf[4:6], uint16(bl))
	buf[6] = rpcTypeResponse
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
func rpcDecodeBody(buf []byte) []byte {
	bl := rpcDecodeBodyLen(buf)
	out := make([]byte, bl)
	copy(out, buf[rpcBodyOff:rpcBodyOff+bl])
	return out
}

// ── RpcClient ────────────────────────────────────────────────────────────────

// RpcClient wraps a Client with correlation-id based request/response
// matching, so multiple goroutines can call Call() concurrently.
type RpcClient struct {
	cl      *Client
	muSend  sync.Mutex     // serialise Send (single-writer queue)
	pending sync.Map       // id → chan []byte
	nextID  atomic.Uint32
	sendBuf []byte
	sem     chan struct{}  // limits in-flight requests below buffer ring depth
}

// rpcMaxOutstanding caps in-flight Call()s. Must stay well below the underlying
// TCS WindowSize (256) so the client's allocate cursor never wraps onto a
// buffer slot still holding a pending response from the daemon.
const rpcMaxOutstanding = 64

// NewRpcClient connects to an existing bidirectional channel.
func NewRpcClient(name string) (*RpcClient, error) {
	cfg := Config{Name: name, MsgSize: RpcMsgSize, WindowSize: 256, Blocking: true}
	cl, err := NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("RpcClient: %w", err)
	}

	c := &RpcClient{
		cl:      cl,
		sendBuf: make([]byte, RpcMsgSize),
		sem:     make(chan struct{}, rpcMaxOutstanding),
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
			body := rpcDecodeBody(msg)
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
func (c *RpcClient) Call(method string, body []byte) ([]byte, error) {
	// Cap concurrent in-flight Call()s so the underlying buffer ring
	// cursor never wraps onto a slot still pending a daemon response.
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	id := c.nextID.Add(1)
	ch := make(chan []byte, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	c.muSend.Lock()
	n := rpcEncodeRequest(c.sendBuf, id, method, body)
	err := c.cl.Send(c.sendBuf[:n])
	c.muSend.Unlock()
	if err != nil {
		return nil, err
	}

	resp := <-ch
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
	cfg := Config{Name: name, MsgSize: RpcMsgSize, WindowSize: 256, Blocking: true}
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
			body := rpcDecodeBody(msg)

			go func() {
				respBody := handler(method, body)
				buf := make([]byte, RpcMsgSize)
				n := rpcEncodeResponse(buf, id, respBody)

				s.muSend.Lock()
				s.sv.Send(buf[:n])
				s.muSend.Unlock()
			}()
		}
	}()

	return s, nil
}

func (s *RpcServer) Close() { s.sv.Close() }
