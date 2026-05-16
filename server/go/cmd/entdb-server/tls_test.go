package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestBuildServerTLSConfigPlaintextAllowed(t *testing.T) {
	tlsCfg, enabled, reloader, err := buildServerTLSConfig(serverTLSConfig{})
	if err != nil {
		t.Fatalf("buildServerTLSConfig: %v", err)
	}
	if enabled {
		t.Fatal("enabled = true, want false")
	}
	if tlsCfg != nil {
		t.Fatalf("tlsCfg = %#v, want nil", tlsCfg)
	}
	if reloader != nil {
		t.Fatalf("reloader = %#v, want nil", reloader)
	}
}

func TestBuildServerTLSConfigRequireTLSRejectsPlaintext(t *testing.T) {
	_, _, _, err := buildServerTLSConfig(serverTLSConfig{requireTLS: true})
	if err == nil {
		t.Fatal("buildServerTLSConfig err = nil, want require TLS error")
	}
	if !strings.Contains(err.Error(), "--require-tls") {
		t.Fatalf("err = %q, want --require-tls", err)
	}
}

func TestBuildServerTLSConfigServerTLS(t *testing.T) {
	certFile, keyFile, _ := writeTLSFixture(t)

	tlsCfg, enabled, reloader, err := buildServerTLSConfig(serverTLSConfig{
		certFile: certFile,
		keyFile:  keyFile,
	})
	if err != nil {
		t.Fatalf("buildServerTLSConfig: %v", err)
	}
	if !enabled {
		t.Fatal("enabled = false, want true")
	}
	if reloader == nil {
		t.Fatal("reloader = nil, want non-nil")
	}
	if tlsCfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion = %x, want TLS 1.3", tlsCfg.MinVersion)
	}
	if got := len(tlsCfg.Certificates); got != 1 {
		t.Fatalf("certificates len = %d, want 1", got)
	}
	if tlsCfg.ClientAuth != tls.NoClientCert {
		t.Fatalf("ClientAuth = %v, want NoClientCert", tlsCfg.ClientAuth)
	}
}

func TestBuildServerTLSConfigOptionalClientCertVerification(t *testing.T) {
	certFile, keyFile, caFile := writeTLSFixture(t)

	tlsCfg, enabled, _, err := buildServerTLSConfig(serverTLSConfig{
		certFile: certFile,
		keyFile:  keyFile,
		caFile:   caFile,
	})
	if err != nil {
		t.Fatalf("buildServerTLSConfig: %v", err)
	}
	if !enabled {
		t.Fatal("enabled = false, want true")
	}
	if tlsCfg.ClientCAs == nil {
		t.Fatal("ClientCAs = nil, want CA pool")
	}
	if tlsCfg.ClientAuth != tls.VerifyClientCertIfGiven {
		t.Fatalf("ClientAuth = %v, want VerifyClientCertIfGiven", tlsCfg.ClientAuth)
	}
}

func TestBuildServerTLSConfigRequiresClientCert(t *testing.T) {
	certFile, keyFile, caFile := writeTLSFixture(t)

	tlsCfg, enabled, _, err := buildServerTLSConfig(serverTLSConfig{
		certFile:          certFile,
		keyFile:           keyFile,
		caFile:            caFile,
		requireClientCert: true,
	})
	if err != nil {
		t.Fatalf("buildServerTLSConfig: %v", err)
	}
	if !enabled {
		t.Fatal("enabled = false, want true")
	}
	if tlsCfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("ClientAuth = %v, want RequireAndVerifyClientCert", tlsCfg.ClientAuth)
	}
}

func TestBuildServerTLSConfigRequireClientCertNeedsCA(t *testing.T) {
	certFile, keyFile, _ := writeTLSFixture(t)

	_, _, _, err := buildServerTLSConfig(serverTLSConfig{
		certFile:          certFile,
		keyFile:           keyFile,
		requireClientCert: true,
	})
	if err == nil {
		t.Fatal("buildServerTLSConfig err = nil, want --tls-ca error")
	}
	if !strings.Contains(err.Error(), "--tls-ca") {
		t.Fatalf("err = %q, want --tls-ca", err)
	}
}

func TestBuildServerTLSConfigTLS12Compat(t *testing.T) {
	certFile, keyFile, _ := writeTLSFixture(t)

	tlsCfg, enabled, _, err := buildServerTLSConfig(serverTLSConfig{
		certFile:   certFile,
		keyFile:    keyFile,
		minVersion: "1.2",
	})
	if err != nil {
		t.Fatalf("buildServerTLSConfig: %v", err)
	}
	if !enabled {
		t.Fatal("enabled = false, want true")
	}
	if tlsCfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %x, want TLS 1.2", tlsCfg.MinVersion)
	}
	if len(tlsCfg.CipherSuites) == 0 {
		t.Fatal("CipherSuites empty, want pinned modern TLS 1.2 suites")
	}
}

func TestBuildServerTLSConfigRejectsInvalidMinVersion(t *testing.T) {
	certFile, keyFile, _ := writeTLSFixture(t)

	_, _, _, err := buildServerTLSConfig(serverTLSConfig{
		certFile:   certFile,
		keyFile:    keyFile,
		minVersion: "1.1",
	})
	if err == nil {
		t.Fatal("buildServerTLSConfig err = nil, want invalid version error")
	}
	if !strings.Contains(err.Error(), "--tls-min-version") {
		t.Fatalf("err = %q, want --tls-min-version", err)
	}
}

func TestBuildServerTLSConfigReloadsCertificate(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile, _ := writeTLSFixtureInDir(t, dir, 2)
	tlsCfg, enabled, reloader, err := buildServerTLSConfig(serverTLSConfig{
		certFile: certFile,
		keyFile:  keyFile,
	})
	if err != nil {
		t.Fatalf("buildServerTLSConfig: %v", err)
	}
	if !enabled || reloader == nil {
		t.Fatalf("enabled=%v reloader=%v, want TLS reloader", enabled, reloader)
	}
	if got := leafSerial(t, tlsCfg); got != 2 {
		t.Fatalf("initial cert serial = %d, want 2", got)
	}

	writeTLSFixtureInDir(t, dir, 99)
	if err := reloader.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if got := leafSerial(t, tlsCfg); got != 99 {
		t.Fatalf("reloaded cert serial = %d, want 99", got)
	}
}

func TestMTLSHandshakeRequiresClientCertificate(t *testing.T) {
	certFile, keyFile, caFile, clientCertFile, clientKeyFile := writeMTLSFixture(t)
	tlsCfg, enabled, _, err := buildServerTLSConfig(serverTLSConfig{
		certFile:          certFile,
		keyFile:           keyFile,
		caFile:            caFile,
		requireClientCert: true,
	})
	if err != nil {
		t.Fatalf("buildServerTLSConfig: %v", err)
	}
	if !enabled {
		t.Fatal("TLS disabled, want enabled")
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	srv := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsCfg)))
	healthpb.RegisterHealthServer(srv, health.NewServer())
	go func() {
		_ = srv.Serve(lis)
	}()
	defer srv.Stop()

	if err := callHealth(t, lis.Addr().String(), insecure.NewCredentials()); err == nil {
		t.Fatal("plaintext health call succeeded, want TLS handshake failure")
	}

	rootPool, err := loadCertPool(caFile)
	if err != nil {
		t.Fatalf("loadCertPool: %v", err)
	}
	noClientCert := credentials.NewTLS(&tls.Config{
		RootCAs:    rootPool,
		ServerName: "localhost",
		MinVersion: tls.VersionTLS13,
	})
	if err := callHealth(t, lis.Addr().String(), noClientCert); err == nil {
		t.Fatal("TLS health call without client cert succeeded, want mTLS failure")
	}

	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		t.Fatalf("LoadX509KeyPair client: %v", err)
	}
	withClientCert := credentials.NewTLS(&tls.Config{
		RootCAs:      rootPool,
		ServerName:   "localhost",
		Certificates: []tls.Certificate{clientCert},
		MinVersion:   tls.VersionTLS13,
	})
	if err := callHealth(t, lis.Addr().String(), withClientCert); err != nil {
		t.Fatalf("mTLS health call with client cert failed: %v", err)
	}
}

func callHealth(t *testing.T, addr string, creds credentials.TransportCredentials) error {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	return err
}

func writeTLSFixture(t *testing.T) (certFile, keyFile, caFile string) {
	t.Helper()
	return writeTLSFixtureInDir(t, t.TempDir(), 2)
}

func writeTLSFixtureInDir(t *testing.T, dir string, serverSerial int64) (certFile, keyFile, caFile string) {
	t.Helper()
	caKey := newECDSAKey(t)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "entdb-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("CreateCertificate(ca): %v", err)
	}

	serverKey := newECDSAKey(t)
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(serverSerial),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("CreateCertificate(server): %v", err)
	}

	caFile = filepath.Join(dir, "ca.pem")
	certFile = filepath.Join(dir, "server.pem")
	keyFile = filepath.Join(dir, "server-key.pem")
	writePEM(t, caFile, "CERTIFICATE", caDER)
	writePEM(t, certFile, "CERTIFICATE", serverDER)
	writeECPrivateKey(t, keyFile, serverKey)
	return certFile, keyFile, caFile
}

func writeMTLSFixture(t *testing.T) (serverCertFile, serverKeyFile, caFile, clientCertFile, clientKeyFile string) {
	t.Helper()
	dir := t.TempDir()
	caKey := newECDSAKey(t)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "entdb-mtls-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("CreateCertificate(ca): %v", err)
	}
	serverKey := newECDSAKey(t)
	serverDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}, caTemplate, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("CreateCertificate(server): %v", err)
	}
	clientKey := newECDSAKey(t)
	clientDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}, caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("CreateCertificate(client): %v", err)
	}
	caFile = filepath.Join(dir, "ca.pem")
	serverCertFile = filepath.Join(dir, "server.pem")
	serverKeyFile = filepath.Join(dir, "server-key.pem")
	clientCertFile = filepath.Join(dir, "client.pem")
	clientKeyFile = filepath.Join(dir, "client-key.pem")
	writePEM(t, caFile, "CERTIFICATE", caDER)
	writePEM(t, serverCertFile, "CERTIFICATE", serverDER)
	writeECPrivateKey(t, serverKeyFile, serverKey)
	writePEM(t, clientCertFile, "CERTIFICATE", clientDER)
	writeECPrivateKey(t, clientKeyFile, clientKey)
	return serverCertFile, serverKeyFile, caFile, clientCertFile, clientKeyFile
}

func leafSerial(t *testing.T, tlsCfg *tls.Config) int64 {
	t.Helper()
	cfg, err := tlsCfg.GetConfigForClient(&tls.ClientHelloInfo{})
	if err != nil {
		t.Fatalf("GetConfigForClient: %v", err)
	}
	if len(cfg.Certificates) != 1 || len(cfg.Certificates[0].Certificate) == 0 {
		t.Fatalf("GetConfigForClient certificates = %#v, want one leaf", cfg.Certificates)
	}
	leaf, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	return leaf.SerialNumber.Int64()
}

func newECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return key
}

func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%s): %v", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := pem.Encode(f, &pem.Block{Type: typ, Bytes: der}); err != nil {
		t.Fatalf("pem.Encode(%s): %v", path, err)
	}
}

func writeECPrivateKey(t *testing.T, path string, key *ecdsa.PrivateKey) {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey: %v", err)
	}
	writePEM(t, path, "EC PRIVATE KEY", der)
}
