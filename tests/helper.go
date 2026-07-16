package tests

import (
	"context"
	"net"
	"testing"
	"time"
)

func requireServer(t *testing.T, addr string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.Errorf("skip integration test: server %s unavailable: %v", addr, err)
	}
	_ = conn.Close()
}
