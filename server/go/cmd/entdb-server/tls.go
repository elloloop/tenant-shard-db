package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"sync"

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

func grpcServerTLSOptions(cfg serverTLSConfig) ([]grpc.ServerOption, bool, *tlsReloader, error) {
	tlsCfg, enabled, reloader, err := buildServerTLSConfig(cfg)
	if err != nil {
		return nil, false, nil, err
	}
	if !enabled {
		return nil, false, nil, nil
	}
	return []grpc.ServerOption{grpc.Creds(credentials.NewTLS(tlsCfg))}, true, reloader, nil
}

func buildServerTLSConfig(cfg serverTLSConfig) (*tls.Config, bool, *tlsReloader, error) {
	certFile := strings.TrimSpace(cfg.certFile)
	keyFile := strings.TrimSpace(cfg.keyFile)
	caFile := strings.TrimSpace(cfg.caFile)

	tlsRequested := certFile != "" || keyFile != "" || caFile != ""
	if !tlsRequested {
		if cfg.requireTLS {
			return nil, false, nil, fmt.Errorf("--require-tls requires --tls-cert and --tls-key")
		}
		if cfg.requireClientCert {
			return nil, false, nil, fmt.Errorf("--require-client-cert requires TLS and --tls-ca")
		}
		return nil, false, nil, nil
	}
	if certFile == "" || keyFile == "" {
		return nil, false, nil, fmt.Errorf("TLS requires both --tls-cert and --tls-key")
	}

	minVersion, err := parseTLSMinVersion(cfg.minVersion)
	if err != nil {
		return nil, false, nil, err
	}
	clientAuth := tls.NoClientCert
	if caFile != "" {
		clientAuth = tls.VerifyClientCertIfGiven
	}
	if cfg.requireClientCert {
		if caFile == "" {
			return nil, false, nil, fmt.Errorf("--require-client-cert requires --tls-ca")
		}
		clientAuth = tls.RequireAndVerifyClientCert
	}

	reloader, err := newTLSReloader(serverTLSConfig{
		certFile:          certFile,
		keyFile:           keyFile,
		caFile:            caFile,
		minVersion:        cfg.minVersion,
		requireTLS:        cfg.requireTLS,
		requireClientCert: cfg.requireClientCert,
	}, minVersion, clientAuth)
	if err != nil {
		return nil, false, nil, err
	}
	tlsCfg := reloader.currentConfig()
	tlsCfg.GetConfigForClient = reloader.GetConfigForClient
	return tlsCfg, true, reloader, nil
}

type tlsReloader struct {
	cfg        serverTLSConfig
	minVersion uint16
	clientAuth tls.ClientAuthType

	mu        sync.RWMutex
	cert      tls.Certificate
	clientCAs *x509.CertPool
}

func newTLSReloader(cfg serverTLSConfig, minVersion uint16, clientAuth tls.ClientAuthType) (*tlsReloader, error) {
	r := &tlsReloader{
		cfg:        cfg,
		minVersion: minVersion,
		clientAuth: clientAuth,
	}
	if err := r.Reload(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *tlsReloader) Reload() error {
	cert, err := tls.LoadX509KeyPair(r.cfg.certFile, r.cfg.keyFile)
	if err != nil {
		return fmt.Errorf("load server certificate/key: %w", err)
	}
	var pool *x509.CertPool
	if strings.TrimSpace(r.cfg.caFile) != "" {
		pool, err = loadCertPool(r.cfg.caFile)
		if err != nil {
			return err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cert = cert
	r.clientCAs = pool
	return nil
}

func (r *tlsReloader) GetConfigForClient(*tls.ClientHelloInfo) (*tls.Config, error) {
	return r.currentConfig(), nil
}

func (r *tlsReloader) currentConfig() *tls.Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return &tls.Config{
		MinVersion:   r.minVersion,
		Certificates: []tls.Certificate{r.cert},
		ClientCAs:    r.clientCAs,
		ClientAuth:   r.clientAuth,
		CipherSuites: modernCipherSuites(),
	}
}

func modernCipherSuites() []uint16 {
	return []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	}
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
