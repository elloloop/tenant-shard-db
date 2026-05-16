package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildServerTLSConfigPlaintextAllowed(t *testing.T) {
	tlsCfg, enabled, err := buildServerTLSConfig(serverTLSConfig{})
	if err != nil {
		t.Fatalf("buildServerTLSConfig: %v", err)
	}
	if enabled {
		t.Fatal("enabled = true, want false")
	}
	if tlsCfg != nil {
		t.Fatalf("tlsCfg = %#v, want nil", tlsCfg)
	}
}

func TestBuildServerTLSConfigRequireTLSRejectsPlaintext(t *testing.T) {
	_, _, err := buildServerTLSConfig(serverTLSConfig{requireTLS: true})
	if err == nil {
		t.Fatal("buildServerTLSConfig err = nil, want require TLS error")
	}
	if !strings.Contains(err.Error(), "--require-tls") {
		t.Fatalf("err = %q, want --require-tls", err)
	}
}

func TestBuildServerTLSConfigServerTLS(t *testing.T) {
	certFile, keyFile, _ := writeTLSFixture(t)

	tlsCfg, enabled, err := buildServerTLSConfig(serverTLSConfig{
		certFile: certFile,
		keyFile:  keyFile,
	})
	if err != nil {
		t.Fatalf("buildServerTLSConfig: %v", err)
	}
	if !enabled {
		t.Fatal("enabled = false, want true")
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

	tlsCfg, enabled, err := buildServerTLSConfig(serverTLSConfig{
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

	tlsCfg, enabled, err := buildServerTLSConfig(serverTLSConfig{
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

	_, _, err := buildServerTLSConfig(serverTLSConfig{
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

	tlsCfg, enabled, err := buildServerTLSConfig(serverTLSConfig{
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

	_, _, err := buildServerTLSConfig(serverTLSConfig{
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

func writeTLSFixture(t *testing.T) (certFile, keyFile, caFile string) {
	t.Helper()
	dir := t.TempDir()

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
		SerialNumber: big.NewInt(2),
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
