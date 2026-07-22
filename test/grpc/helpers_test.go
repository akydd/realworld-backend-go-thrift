//go:build integration

package grpc_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"realworld-backend-go/api/proto/gen/pb"
)

func TestMain(m *testing.M) {
	_ = godotenv.Load("../../.env_test")
	// cert paths in .env_test are relative to the repo root; tests run from test/grpc/
	for _, key := range []string{"GRPC_TLS_CA", "GRPC_CLIENT_CERT", "GRPC_CLIENT_KEY"} {
		if v := os.Getenv(key); v != "" {
			os.Setenv(key, "../../"+v) //nolint:errcheck
		}
	}
	os.Exit(m.Run())
}

func grpcAddr() string {
	if h := os.Getenv("GRPC_HOST"); h != "" {
		return h
	}
	return "localhost:8098"
}

func dial(t *testing.T) *grpc.ClientConn {
	t.Helper()

	caCertFile := os.Getenv("GRPC_TLS_CA")
	clientCertFile := os.Getenv("GRPC_CLIENT_CERT")
	clientKeyFile := os.Getenv("GRPC_CLIENT_KEY")

	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		t.Fatalf("error reading CA cert file: %v", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		t.Fatalf("failed to append CA cert to pool")
	}

	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		t.Fatalf("error loading clientCert: %v", err)
	}

	c := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS13,
	}
	creds := credentials.NewTLS(c)

	conn, err := grpc.NewClient(grpcAddr(), grpc.WithTransportCredentials(creds))
	if err != nil {
		t.Fatalf("dial %s: %v", grpcAddr(), err)
	}

	t.Cleanup(func() { conn.Close() })
	return conn
}

func withToken(ctx context.Context, token string) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Token "+token))
}

func genUID() string {
	return fmt.Sprintf("%d%04d", time.Now().Unix(), time.Now().Nanosecond()%10000)
}

func nullableStr(s string) *pb.NullableString {
	return &pb.NullableString{Value: s}
}

func clearNullable() *pb.NullableString {
	return &pb.NullableString{}
}

func strPtr(s string) *string { return &s }
