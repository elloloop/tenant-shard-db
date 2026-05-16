package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type serverTLSConfig struct {
	certFile          string
	keyFile           string
	caFile            string
	minVersion        string
	requireTLS        bool
	requireClientCert bool
}

func grpcServerTLSOptions(cfg serverTLSConfig) ([]grpc.ServerOption, bool, error) {
	tlsCfg, enabled, err := buildServerTLSConfig(cfg)
	if err != nil {
		return nil, false, err
	}
	if !enabled {
		return nil, false, nil
	}
	return []grpc.ServerOption{grpc.Creds(credentials.NewTLS(tlsCfg))}, true, nil
}

func buildServerTLSConfig(cfg serverTLSConfig) (*tls.Config, bool, error) {
	certFile := strings.TrimSpace(cfg.certFile)
	keyFile := strings.TrimSpace(cfg.keyFile)
	caFile := strings.TrimSpace(cfg.caFile)

	tlsRequested := certFile != "" || keyFile != "" || caFile != ""
	if !tlsRequested {
		if cfg.requireTLS {
			return nil, false, fmt.Errorf("--require-tls requires --tls-cert and --tls-key")
		}
		if cfg.requireClientCert {
			return nil, false, fmt.Errorf("--require-client-cert requires TLS and --tls-ca")
		}
		return nil, false, nil
	}
	if certFile == "" || keyFile == "" {
		return nil, false, fmt.Errorf("TLS requires both --tls-cert and --tls-key")
	}

	minVersion, err := parseTLSMinVersion(cfg.minVersion)
	if err != nil {
		return nil, false, err
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, false, fmt.Errorf("load server certificate/key: %w", err)
	}

	tlsCfg := &tls.Config{
		MinVersion:   minVersion,
		Certificates: []tls.Certificate{cert},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
	}

	if caFile != "" {
		pool, err := loadCertPool(caFile)
		if err != nil {
			return nil, false, err
		}
		tlsCfg.ClientCAs = pool
		tlsCfg.ClientAuth = tls.VerifyClientCertIfGiven
	}
	if cfg.requireClientCert {
		if caFile == "" {
			return nil, false, fmt.Errorf("--require-client-cert requires --tls-ca")
		}
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsCfg, true, nil
}

func parseTLSMinVersion(version string) (uint16, error) {
	switch strings.ToLower(strings.TrimSpace(version)) {
	case "", "1.3", "tls1.3", "tls13":
		return tls.VersionTLS13, nil
	case "1.2", "tls1.2", "tls12":
		return tls.VersionTLS12, nil
	default:
		return 0, fmt.Errorf("unsupported --tls-min-version %q (want 1.3 or 1.2)", version)
	}
}

func loadCertPool(path string) (*x509.CertPool, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read --tls-ca: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("parse --tls-ca: no certificates found")
	}
	return pool, nil
}

func clientAuthMode(caFile string, requireClientCert bool) string {
	switch {
	case requireClientCert:
		return "require-and-verify"
	case strings.TrimSpace(caFile) != "":
		return "verify-if-present"
	default:
		return "none"
	}
}
