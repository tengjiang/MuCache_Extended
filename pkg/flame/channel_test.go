package flame_test

import (
	"bytes"
	"os/exec"
	"testing"
	"time"

	"github.com/DKW2/MuCache_Extended/pkg/flame"
)

func TestChannelRoundtrip(t *testing.T) {
	const channelName = "test_flame_roundtrip"
	const ready = "/tmp/flame_test_ready"

	// Remove any leftover shm regions from a previous failed run
	exec.Command("sh", "-c",
		"rm -f /dev/shm/"+channelName+"_cd /dev/shm/"+channelName+"_ds "+ready).Run()

	// Start the flame daemon as a subprocess
	daemon := exec.Command("/mydata/flame-benchmark/bin/flame_daemon",
		"--channel-name", channelName,
		"--msg-size", "1280",
		"--capacity", "256",
		"--ready-path", ready,
	)
	if err := daemon.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer daemon.Process.Kill()

	// Wait for ready file
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := exec.Command("test", "-f", ready).Output(); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cfg := flame.Config{
		Name:     channelName,
		MsgSize:  1280,
		Capacity: 256,
		Doorbell: false,
	}

	// Encode a StartRequest
	buf := make([]byte, flame.MsgSize)
	flame.EncodeStart(buf, "deadbeef", "service1")

	// Writer
	w, err := flame.NewWriter(cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	// Reader (receives into a channel)
	got := make(chan []byte, 1)
	r, err := flame.NewReader(cfg, func(msg []byte) {
		cp := make([]byte, len(msg))
		copy(cp, msg)
		select {
		case got <- cp:
		default:
		}
	})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	// Send
	w.Send(buf)

	// Receive in a goroutine
	go r.Recv()

	select {
	case msg := <-got:
		if !bytes.Equal(msg[:len(buf)], buf) {
			t.Errorf("message mismatch: got %x, want %x", msg[:16], buf[:16])
		}
		// Decode and verify
		decoded, err := flame.Decode(msg)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		start, ok := decoded.(flame.StartDecoded)
		if !ok {
			t.Fatalf("expected StartDecoded, got %T", decoded)
		}
		if start.CallArgs != "deadbeef" {
			t.Errorf("CallArgs: got %q, want %q", start.CallArgs, "deadbeef")
		}
		if start.AppName != "service1" {
			t.Errorf("AppName: got %q, want %q", start.AppName, "service1")
		}
		t.Logf("round-trip OK: %+v", start)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}
