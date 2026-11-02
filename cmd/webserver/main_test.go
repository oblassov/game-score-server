package main

import (
	"net"
	"net/http"
	"testing"
)

func TestNewHTTPServerListensOnPort5000(t *testing.T) {
	srv := newHTTPServer(http.NotFoundHandler())

	_, port, err := net.SplitHostPort(srv.Addr)
	if err != nil {
		t.Fatalf("server address %q is not host:port: %v", srv.Addr, err)
	}

	if port != "5000" {
		t.Errorf("got port %q, want %q", port, "5000")
	}
}
