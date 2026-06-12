package twoserivces

type ReadRequest struct {
	K int `json:"k"`
}

type ReadResponse struct {
	V int `json:"v"`
}

type ReadBulkRequest struct {
	Ks []int `json:"ks"`
}

type ReadBulkResponse struct {
	Vs []int `json:"vs"`
}

type WriteRequest struct {
	K int `json:"k"`
	V int `json:"v"`
}

type WriteBulkRequest struct {
	Ks []int `json:"ks"`
	Vs []int `json:"vs"`
}

type HitOrMissRequest struct {
	K       int     `json:"k"`
	HitRate float32 `json:"hit_rate"` // [0.0, 1.0)
}

// BlobReadRequest asks the chain to flow back Size bytes of arbitrary
// payload. Used by the payload-size sweep — varies the actual bytes that
// move through the chain rather than just RpcMsgSize (the shm slot
// capacity).
type BlobReadRequest struct {
	Size int `json:"size"`
}

// BlobReadResponse carries the requested Size bytes. The chain's backend
// allocates Data = make([]byte, Size) per call; intermediate services
// pass it through, forcing a marshal+unmarshal at every hop so the bytes
// are actually touched on the way back.
type BlobReadResponse struct {
	Data []byte `json:"data"`
}
