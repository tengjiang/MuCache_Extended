//go:build flame
// +build flame

package cm

import (
	"github.com/DKW2/MuCache_Extended/pkg/common"
	"github.com/DKW2/MuCache_Extended/pkg/flame"
	"github.com/golang/glog"
)

// StartFlame launches FlameProxy in a background goroutine.
// Only runs when FLAME_CHANNEL env var is set (CM-based flame mode).
// No-op for service-to-service flame mode which uses FLAME_UPSTREAM/DOWNSTREAM.
func StartFlame() {
	if common.FlameChannelName == "" {
		return
	}
	go FlameProxy()
}

// FlameProxy drains the work queue (WQ) and forwards each message to the CM
// via the flame RPC channel (shared memory).
func FlameProxy() {
	cfg := flame.Config{
		Name:     common.FlameChannelName,
		MsgSize:  flame.MsgSize,
		Capacity: 256,
		Doorbell: false, // polling default
	}

	var writer *flame.Writer
	var err error
	for {
		writer, err = flame.NewWriter(cfg)
		if err == nil {
			break
		}
		glog.Warningf("[FlameProxy] waiting for daemon channel %q: %v", cfg.Name, err)
	}
	defer writer.Close()

	glog.Infof("[FlameProxy] connected to flame channel %q", cfg.Name)

	buf := make([]byte, flame.MsgSize)

	for {
		if len(WQ) >= QueueSize/2 {
			glog.Warningf("[FlameProxy] queue size: %d", len(WQ))
		}
		req := <-WQ

		// Zero the buffer so stale bytes from a larger previous message
		// don't leak into the fixed-size frame.
		for i := range buf {
			buf[i] = 0
		}

		switch r := req.(type) {
		case *StartRequest:
			flame.EncodeStart(buf, string(r.CallArgs), common.MyName)
		case *EndRequest:
			kd := make([]string, len(r.KeyDeps))
			for i, k := range r.KeyDeps {
				kd[i] = string(k)
			}
			cd := make([]string, len(r.CallDeps))
			for i, c := range r.CallDeps {
				cd[i] = string(c)
			}
			flame.EncodeEnd(buf,
				string(r.CallArgs),
				string(r.Caller),
				kd, cd,
				[]byte(r.ReturnVal))
		case *InvalidateKeyRequest:
			flame.EncodeInvKey(buf, string(r.Key), r.FromCM)
		default:
			glog.Warningf("[FlameProxy] unknown request type %T, dropping", req)
			continue
		}

		writer.Send(buf)
	}
}
