//go:build flame
// +build flame

package cm

import (
	"github.com/DKW2/MuCache_Extended/pkg/cm"
	"github.com/DKW2/MuCache_Extended/pkg/common"
	"github.com/DKW2/MuCache_Extended/pkg/flame"
	"github.com/golang/glog"
)

// ServeFlame starts a goroutine that reads from the flame channel and feeds
// decoded messages into the CM work queue (cm.WQ).
// Call once from the CM's main startup path (alongside or instead of ServeHttp
// for the start/end/inv endpoints).
func ServeFlame(cfg *Config) {
	fCfg := flame.Config{
		Name:     common.FlameChannelName,
		MsgSize:  flame.MsgSize,
		Capacity: 256,
		Doorbell: false,
	}

	reader, err := flame.NewReader(fCfg, func(buf []byte) {
		msg, err := flame.Decode(buf)
		if err != nil {
			glog.Warningf("[ServeFlame] decode error: %v", err)
			return
		}
		switch m := msg.(type) {
		case flame.StartDecoded:
			cm.WQ <- cm.StartRequest{CallArgs: cm.CallArgs(m.CallArgs)}

		case flame.EndDecoded:
			kd := make([]cm.Key, len(m.KeyDeps))
			for i, k := range m.KeyDeps {
				kd[i] = cm.Key(k)
			}
			cd := make([]cm.CallArgs, len(m.CallDeps))
			for i, c := range m.CallDeps {
				cd[i] = cm.CallArgs(c)
			}
			cm.WQ <- cm.EndRequest{
				CallArgs:  cm.CallArgs(m.CallArgs),
				Caller:    cm.ServiceName(m.Caller),
				KeyDeps:   kd,
				CallDeps:  cd,
				ReturnVal: cm.ReturnVal(m.RetVal),
			}

		case flame.InvKeyDecoded:
			cm.WQ <- cm.InvalidateKeyRequest{
				Key:    cm.Key(m.Key),
				FromCM: m.FromCM,
			}
		}
	})

	if err != nil {
		glog.Fatalf("[ServeFlame] failed to connect to flame channel %q: %v",
			fCfg.Name, err)
	}

	glog.Infof("[ServeFlame] listening on flame channel %q", fCfg.Name)

	go func() {
		for {
			reader.Recv()
		}
	}()
}
