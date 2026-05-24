// SPDX-License-Identifier: AGPL-3.0-only

package wal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IBM/sarama"
)

func TestApplySecurity_PlainSASL(t *testing.T) {
	cfg := KafkaConfig{
		SASLEnable:    true,
		SASLMechanism: "PLAIN",
		SASLUsername:  "user",
		SASLPassword:  "pass",
	}
	c := sarama.NewConfig()
	if err := cfg.applySecurity(c); err != nil {
		t.Fatalf("applySecurity: %v", err)
	}
	if !c.Net.SASL.Enable {
		t.Fatal("SASL not enabled")
	}
	if c.Net.SASL.Mechanism != sarama.SASLTypePlaintext {
		t.Errorf("mechanism = %v, want PLAIN", c.Net.SASL.Mechanism)
	}
	if c.Net.SASL.User != "user" || c.Net.SASL.Password != "pass" {
		t.Errorf("creds = %q/%q", c.Net.SASL.User, c.Net.SASL.Password)
	}
	if c.Net.TLS.Enable {
		t.Error("TLS should be off when not requested")
	}
}

func TestApplySecurity_DefaultMechanismIsPlain(t *testing.T) {
	cfg := KafkaConfig{SASLEnable: true, SASLUsername: "u", SASLPassword: "p"}
	c := sarama.NewConfig()
	if err := cfg.applySecurity(c); err != nil {
		t.Fatalf("applySecurity: %v", err)
	}
	if c.Net.SASL.Mechanism != sarama.SASLTypePlaintext {
		t.Errorf("empty mechanism should default to PLAIN, got %v", c.Net.SASL.Mechanism)
	}
}

func TestApplySecurity_SCRAM(t *testing.T) {
	for _, tc := range []struct {
		mech string
		want sarama.SASLMechanism
	}{
		{"SCRAM-SHA-256", sarama.SASLTypeSCRAMSHA256},
		{"scram-sha-512", sarama.SASLTypeSCRAMSHA512}, // case-insensitive
	} {
		c := sarama.NewConfig()
		cfg := KafkaConfig{SASLEnable: true, SASLMechanism: tc.mech, SASLUsername: "u", SASLPassword: "p"}
		if err := cfg.applySecurity(c); err != nil {
			t.Fatalf("%s: applySecurity: %v", tc.mech, err)
		}
		if c.Net.SASL.Mechanism != tc.want {
			t.Errorf("%s: mechanism = %v, want %v", tc.mech, c.Net.SASL.Mechanism, tc.want)
		}
		if c.Net.SASL.SCRAMClientGeneratorFunc == nil {
			t.Fatalf("%s: SCRAM client generator not set", tc.mech)
		}
		// The generated client must Begin a conversation without error.
		client := c.Net.SASL.SCRAMClientGeneratorFunc()
		if err := client.Begin("u", "p", ""); err != nil {
			t.Errorf("%s: SCRAM Begin: %v", tc.mech, err)
		}
		if _, err := client.Step(""); err != nil {
			t.Errorf("%s: SCRAM first step: %v", tc.mech, err)
		}
	}
}

func TestApplySecurity_UnknownMechanismErrors(t *testing.T) {
	cfg := KafkaConfig{SASLEnable: true, SASLMechanism: "OAUTHBEARER", SASLUsername: "u"}
	if err := cfg.applySecurity(sarama.NewConfig()); err == nil {
		t.Fatal("expected error for unsupported mechanism")
	}
}

func TestApplySecurity_TLSEnable(t *testing.T) {
	cfg := KafkaConfig{TLSEnable: true, TLSInsecureSkipVerify: true}
	c := sarama.NewConfig()
	if err := cfg.applySecurity(c); err != nil {
		t.Fatalf("applySecurity: %v", err)
	}
	if !c.Net.TLS.Enable || c.Net.TLS.Config == nil {
		t.Fatal("TLS not enabled / config nil")
	}
	if !c.Net.TLS.Config.InsecureSkipVerify {
		t.Error("InsecureSkipVerify not propagated")
	}
}

func TestApplySecurity_NoneIsNoop(t *testing.T) {
	c := sarama.NewConfig()
	if err := (KafkaConfig{}).applySecurity(c); err != nil {
		t.Fatalf("applySecurity: %v", err)
	}
	if c.Net.SASL.Enable || c.Net.TLS.Enable {
		t.Error("no security requested but SASL/TLS got enabled")
	}
}

func TestBuildTLSConfig_BadCAFile(t *testing.T) {
	cfg := KafkaConfig{TLSEnable: true, TLSCAFile: filepath.Join(t.TempDir(), "missing.pem")}
	if _, err := cfg.buildTLSConfig(); err == nil {
		t.Fatal("expected error for missing CA file")
	}

	// A file with no valid PEM is also rejected.
	bad := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(bad, []byte("not a pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.TLSCAFile = bad
	if _, err := cfg.buildTLSConfig(); err == nil {
		t.Fatal("expected error for invalid PEM CA file")
	}
}

func TestBuildTLSConfig_MutualTLSRequiresBoth(t *testing.T) {
	cfg := KafkaConfig{TLSEnable: true, TLSClientCertFile: "/tmp/cert.pem"} // key missing
	if _, err := cfg.buildTLSConfig(); err == nil {
		t.Fatal("expected error when only the client cert (no key) is set")
	}
}
