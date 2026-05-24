// SPDX-License-Identifier: AGPL-3.0-only

// Kafka transport security: SASL (PLAIN / SCRAM-SHA-256 / SCRAM-SHA-512)
// and TLS, applied to both the producer and consumer sarama configs
// (#569). sarama does not ship a SCRAM client, so we provide one backed
// by xdg-go/scram — the canonical Go SCRAM implementation.

package wal

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/IBM/sarama"
	"github.com/xdg-go/scram"
)

// applySecurity wires the config's TLS + SASL settings onto a sarama
// config's Net section. Called by both producerConfig and consumerConfig
// so producer and consumer connections share identical auth.
func (cfg KafkaConfig) applySecurity(c *sarama.Config) error {
	if cfg.TLSEnable {
		tlsConf, err := cfg.buildTLSConfig()
		if err != nil {
			return err
		}
		c.Net.TLS.Enable = true
		c.Net.TLS.Config = tlsConf
	}

	if !cfg.SASLEnable {
		return nil
	}
	c.Net.SASL.Enable = true
	c.Net.SASL.User = cfg.SASLUsername
	c.Net.SASL.Password = cfg.SASLPassword

	switch strings.ToUpper(strings.TrimSpace(cfg.SASLMechanism)) {
	case "", "PLAIN":
		c.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	case "SCRAM-SHA-256":
		c.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		c.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
			return &scramClient{hashGen: scram.SHA256}
		}
	case "SCRAM-SHA-512":
		c.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		c.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
			return &scramClient{hashGen: scram.SHA512}
		}
	default:
		return fmt.Errorf("kafka: unsupported SASL mechanism %q (want PLAIN, SCRAM-SHA-256, or SCRAM-SHA-512)", cfg.SASLMechanism)
	}
	return nil
}

// buildTLSConfig assembles a *tls.Config from the cfg's TLS settings:
// system roots by default, plus an optional private-CA bundle and an
// optional client certificate for mutual TLS.
func (cfg KafkaConfig) buildTLSConfig() (*tls.Config, error) {
	t := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.TLSInsecureSkipVerify, //nolint:gosec // opt-in, test-only knob
	}

	if cfg.TLSCAFile != "" {
		pem, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("kafka: read TLS CA file %q: %w", cfg.TLSCAFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("kafka: TLS CA file %q contains no valid PEM certificates", cfg.TLSCAFile)
		}
		t.RootCAs = pool
	}

	switch {
	case cfg.TLSClientCertFile != "" && cfg.TLSClientKeyFile != "":
		cert, err := tls.LoadX509KeyPair(cfg.TLSClientCertFile, cfg.TLSClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("kafka: load client certificate: %w", err)
		}
		t.Certificates = []tls.Certificate{cert}
	case cfg.TLSClientCertFile != "" || cfg.TLSClientKeyFile != "":
		return nil, fmt.Errorf("kafka: mutual TLS requires both TLSClientCertFile and TLSClientKeyFile")
	}

	return t, nil
}

// scramClient adapts xdg-go/scram to sarama's SCRAMClient interface.
type scramClient struct {
	hashGen      scram.HashGeneratorFcn
	conversation *scram.ClientConversation
}

func (s *scramClient) Begin(userName, password, authzID string) error {
	client, err := s.hashGen.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	s.conversation = client.NewConversation()
	return nil
}

func (s *scramClient) Step(challenge string) (string, error) {
	return s.conversation.Step(challenge)
}

func (s *scramClient) Done() bool {
	return s.conversation.Done()
}
