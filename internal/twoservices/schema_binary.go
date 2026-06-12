package twoserivces

import (
	"encoding/binary"
	"errors"
)

// Binary encodings for the chain Req/Resp types — fixed-layout little-endian
// int64 fields, no allocation overhead, no reflection.
//
// Implementing encoding.BinaryMarshaler/BinaryUnmarshaler signals to
// pkg/invoke and pkg/flame's WrapHandler to use this format on the flame
// path, bypassing JSON. JSON remains the fallback for types that don't
// implement these interfaces.

const (
	readReqSize   = 8  // K int64
	readRespSize  = 8  // V int64
	writeReqSize  = 16 // K, V int64
)

// ── ReadRequest ────────────────────────────────────────────────────────────

func (r ReadRequest) MarshalBinary() ([]byte, error) {
	buf := make([]byte, readReqSize)
	binary.LittleEndian.PutUint64(buf, uint64(r.K))
	return buf, nil
}

func (r *ReadRequest) UnmarshalBinary(data []byte) error {
	if len(data) < readReqSize {
		return errors.New("ReadRequest: short data")
	}
	r.K = int(int64(binary.LittleEndian.Uint64(data)))
	return nil
}

// ── ReadResponse ───────────────────────────────────────────────────────────

func (r ReadResponse) MarshalBinary() ([]byte, error) {
	buf := make([]byte, readRespSize)
	binary.LittleEndian.PutUint64(buf, uint64(r.V))
	return buf, nil
}

func (r *ReadResponse) UnmarshalBinary(data []byte) error {
	if len(data) < readRespSize {
		return errors.New("ReadResponse: short data")
	}
	r.V = int(int64(binary.LittleEndian.Uint64(data)))
	return nil
}

// ── WriteRequest ───────────────────────────────────────────────────────────

func (r WriteRequest) MarshalBinary() ([]byte, error) {
	buf := make([]byte, writeReqSize)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(r.K))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(r.V))
	return buf, nil
}

func (r *WriteRequest) UnmarshalBinary(data []byte) error {
	if len(data) < writeReqSize {
		return errors.New("WriteRequest: short data")
	}
	r.K = int(int64(binary.LittleEndian.Uint64(data[0:8])))
	r.V = int(int64(binary.LittleEndian.Uint64(data[8:16])))
	return nil
}

// ── BlobReadRequest ────────────────────────────────────────────────────────

// 8-byte little-endian int64 Size, no allocation overhead beyond the result.
func (r BlobReadRequest) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(r.Size))
	return buf, nil
}

func (r *BlobReadRequest) UnmarshalBinary(data []byte) error {
	if len(data) < 8 {
		return errors.New("BlobReadRequest: short data")
	}
	r.Size = int(int64(binary.LittleEndian.Uint64(data)))
	return nil
}

// ── BlobReadResponse ───────────────────────────────────────────────────────

// 4-byte little-endian uint32 length prefix + Data bytes. The unmarshal
// path materialises Data as a fresh []byte (a memcpy of len(Data) bytes)
// — this is intentional: the sweep needs each hop to actually touch
// every byte, and the make+copy here is exactly that touch.
func (r BlobReadResponse) MarshalBinary() ([]byte, error) {
	n := len(r.Data)
	buf := make([]byte, 4+n)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(n))
	copy(buf[4:], r.Data)
	return buf, nil
}

func (r *BlobReadResponse) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return errors.New("BlobReadResponse: short header")
	}
	n := int(binary.LittleEndian.Uint32(data[0:4]))
	if len(data) < 4+n {
		return errors.New("BlobReadResponse: truncated data")
	}
	r.Data = make([]byte, n)
	copy(r.Data, data[4:4+n])
	return nil
}
