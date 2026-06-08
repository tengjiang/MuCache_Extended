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
