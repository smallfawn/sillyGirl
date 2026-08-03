package core

import "testing"

func TestGrpcListenAddressUsesEphemeralPortInTests(t *testing.T) {
	t.Setenv("SILLYGIRL_GRPC_ADDR", "")
	if got := grpcListenAddress(); got != "127.0.0.1:0" {
		t.Fatalf("grpcListenAddress() = %q, want 127.0.0.1:0", got)
	}
}

func TestGrpcListenAddressHonorsOverride(t *testing.T) {
	t.Setenv("SILLYGIRL_GRPC_ADDR", "127.0.0.1:54321")
	if got := grpcListenAddress(); got != "127.0.0.1:54321" {
		t.Fatalf("grpcListenAddress() = %q", got)
	}
}
