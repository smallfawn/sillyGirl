package core

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/smallfawn/sillyGirl/proto3/srpc"
	"github.com/smallfawn/sillyGirl/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const grpcRuntimeTokenHeader = "sillygirl-runtime-token"

var grpcRuntimeToken = initialGrpcRuntimeToken()

var grpcRuntimeEndpoint = struct {
	ready   chan struct{}
	once    sync.Once
	mu      sync.RWMutex
	address string
	err     error
}{ready: make(chan struct{})}

// protoc --go_out=. -I. --go-grpc_out=.  bucket.proto
func init() {
	go serveGrpcRuntime()
}

func serveGrpcRuntime() {
	lis, err := net.Listen("tcp", grpcListenAddress())
	if err != nil {
		publishGrpcRuntimeEndpoint("", err)
		log.Printf("gRPC runtime listen failed: %v", err)
		return
	}
	publishGrpcRuntimeEndpoint(grpcDialAddress(lis.Addr().String()), nil)
	s := grpc.NewServer(
		grpc.UnaryInterceptor(grpcUnaryAuthInterceptor),
		grpc.StreamInterceptor(grpcStreamAuthInterceptor),
	)
	srpc.RegisterSillyGirlServiceServer(s, &SillyGirlService{})
	if err := s.Serve(lis); err != nil {
		log.Printf("gRPC runtime serve failed: %v", err)
	}
}

func publishGrpcRuntimeEndpoint(address string, err error) {
	grpcRuntimeEndpoint.mu.Lock()
	grpcRuntimeEndpoint.address = address
	grpcRuntimeEndpoint.err = err
	grpcRuntimeEndpoint.mu.Unlock()
	grpcRuntimeEndpoint.once.Do(func() { close(grpcRuntimeEndpoint.ready) })
}

func grpcListenAddress() string {
	address := strings.TrimSpace(os.Getenv("SILLYGIRL_GRPC_ADDR"))
	if address == "" {
		// gRPC is an internal plugin-runtime transport. An ephemeral loopback
		// port lets multiple SillyGirl instances run on one host without one
		// instance's plugins accidentally connecting to another instance.
		return "127.0.0.1:0"
	}
	return address
}

func initialGrpcRuntimeToken() string {
	token := strings.TrimSpace(os.Getenv("SILLYGIRL_GRPC_TOKEN"))
	if token != "" {
		return token
	}
	return utils.GenUUID()
}

func grpcClientAddress() (string, error) {
	<-grpcRuntimeEndpoint.ready
	grpcRuntimeEndpoint.mu.RLock()
	defer grpcRuntimeEndpoint.mu.RUnlock()
	if grpcRuntimeEndpoint.err != nil {
		return "", grpcRuntimeEndpoint.err
	}
	if grpcRuntimeEndpoint.address == "" {
		return "", errors.New("gRPC runtime address is empty")
	}
	return grpcRuntimeEndpoint.address, nil
}

func grpcDialAddress(address string) string {
	if strings.HasPrefix(address, ":") {
		return "127.0.0.1" + address
	}
	if strings.HasPrefix(address, "0.0.0.0:") || strings.HasPrefix(address, "[::]:") {
		_, port, err := net.SplitHostPort(address)
		if err == nil {
			return "127.0.0.1:" + port
		}
	}
	return address
}

func grpcUnaryAuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if err := requireGrpcRuntimeAuth(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func grpcStreamAuthInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := requireGrpcRuntimeAuth(stream.Context()); err != nil {
		return err
	}
	return handler(srv, stream)
}

func requireGrpcRuntimeAuth(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing grpc metadata")
	}
	values := md.Get(grpcRuntimeTokenHeader)
	if len(values) == 0 {
		return status.Error(codes.Unauthenticated, "missing grpc runtime token")
	}
	if subtle.ConstantTimeCompare([]byte(values[0]), []byte(grpcRuntimeToken)) != 1 {
		return status.Error(codes.Unauthenticated, "invalid grpc runtime token")
	}
	return nil
}

func grpcRuntimeMetadataToken() string {
	if strings.TrimSpace(grpcRuntimeToken) == "" {
		panic(errors.New("grpc runtime token is empty"))
	}
	return grpcRuntimeToken
}
