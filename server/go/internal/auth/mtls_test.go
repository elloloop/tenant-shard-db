package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/url"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

func TestIdentityFromCertificatePrefersURISAN(t *testing.T) {
	uri, err := url.Parse("spiffe://entdb/ns/prod/sa/ingestor")
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	id, ok := IdentityFromCertificate(&x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "ignored-cn"},
		DNSNames:     []string{"ignored.example.com"},
		URIs:         []*url.URL{uri},
	})
	if !ok {
		t.Fatal("IdentityFromCertificate ok=false, want true")
	}
	if id.Method != MethodMTLS || id.Subject != "user:spiffe://entdb/ns/prod/sa/ingestor" {
		t.Fatalf("identity = %+v, want mTLS spiffe user subject", id)
	}
	// A client cert proves which workload connected, not that it is
	// privileged: Authoritative resolves it to a plain user actor, never
	// system/admin (finding #2). Operator-configured elevation is a
	// separate, explicit mechanism.
	if got := Authoritative(WithIdentity(context.Background(), id), User("alice")); got != User("spiffe://entdb/ns/prod/sa/ingestor") {
		t.Fatalf("Authoritative = %v, want SPIFFE user actor (no cert auto-elevation)", got)
	}
}

func TestMTLSUnaryInterceptorInstallsPeerIdentity(t *testing.T) {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(7),
		Subject:      pkix.Name{CommonName: "billing-worker"},
	}
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
		}},
	})
	var captured context.Context
	handler := func(ctx context.Context, req any) (any, error) {
		captured = ctx
		return nil, nil
	}
	if _, err := MTLSPeerIdentityUnaryInterceptor()(ctx, struct{}{}, &grpc.UnaryServerInfo{FullMethod: "/entdb.v1.EntDBService/GetNode"}, handler); err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	id, ok := IdentityFromContext(captured)
	if !ok {
		t.Fatal("IdentityFromContext ok=false, want true")
	}
	if id.Method != MethodMTLS || id.Subject != "user:billing-worker" {
		t.Fatalf("identity = %+v, want billing-worker mTLS identity", id)
	}
}

func TestMTLSUnaryInterceptorLeavesContextWithoutClientCert(t *testing.T) {
	var captured context.Context
	handler := func(ctx context.Context, req any) (any, error) {
		captured = ctx
		return nil, nil
	}
	if _, err := MTLSPeerIdentityUnaryInterceptor()(context.Background(), struct{}{}, &grpc.UnaryServerInfo{FullMethod: "/entdb.v1.EntDBService/GetNode"}, handler); err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	if _, ok := IdentityFromContext(captured); ok {
		t.Fatal("IdentityFromContext ok=true, want no identity without peer cert")
	}
}
