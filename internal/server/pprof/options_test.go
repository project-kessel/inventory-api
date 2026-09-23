package pprof

import "testing"

func TestNewOptionsDefaultsToLoopback(t *testing.T) {
	options := NewOptions()

	if got, want := options.GetListenAddr(), "127.0.0.1:5000"; got != want {
		t.Fatalf("GetListenAddr() = %q, want %q", got, want)
	}
}
