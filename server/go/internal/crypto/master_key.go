package crypto

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	awskmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// MasterKeyConfig configures server bootstrap of the 32-byte EntDB
// encryption master key.
type MasterKeyConfig struct {
	Provider string
	KeyID    string
	DataDir  string

	HTTPClient *http.Client
}

// LoadMasterKey resolves a 32-byte master key from the configured
// provider. Envelope providers persist only ciphertext blobs in DataDir;
// they call the backing KMS on boot to decrypt the plaintext key.
func LoadMasterKey(ctx context.Context, cfg MasterKeyConfig) ([]byte, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	keyID := strings.TrimSpace(cfg.KeyID)
	if provider == "" {
		provider = "file"
	}
	switch provider {
	case "file":
		return loadFileMasterKey(keyID)
	case "aws":
		return loadAWSMasterKey(ctx, cfg)
	case "vault":
		return loadVaultMasterKey(ctx, cfg)
	case "gcp", "azure":
		return nil, fmt.Errorf("crypto: kms provider %q is not implemented in this binary; use file, aws, or vault", provider)
	default:
		return nil, fmt.Errorf("crypto: unsupported kms provider %q (want file|aws|gcp|azure|vault)", cfg.Provider)
	}
}

func loadFileMasterKey(keyID string) ([]byte, error) {
	if keyID == "" {
		return nil, errors.New("crypto: file master key provider requires --kms-key-id path or env:NAME")
	}
	if name, ok := strings.CutPrefix(keyID, "env:"); ok {
		value := os.Getenv(name)
		if value == "" {
			return nil, fmt.Errorf("crypto: master key env var %s is empty or unset", name)
		}
		return parseMasterKeyMaterial([]byte(value))
	}
	data, err := os.ReadFile(keyID)
	if err != nil {
		return nil, fmt.Errorf("crypto: read master key file %q: %w", keyID, err)
	}
	return parseMasterKeyMaterial(data)
}

func parseMasterKeyMaterial(data []byte) ([]byte, error) {
	if len(data) == KeyLength {
		return append([]byte(nil), data...), nil
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == KeyLength {
		return append([]byte(nil), trimmed...), nil
	}
	if key, err := hex.DecodeString(string(trimmed)); err == nil {
		if len(key) != KeyLength {
			return nil, fmt.Errorf("%w: got %d", ErrInvalidMasterKey, len(key))
		}
		return key, nil
	}
	if key, err := base64.StdEncoding.DecodeString(string(trimmed)); err == nil {
		if len(key) != KeyLength {
			return nil, fmt.Errorf("%w: got %d", ErrInvalidMasterKey, len(key))
		}
		return key, nil
	}
	return nil, fmt.Errorf("%w: expected 32 raw bytes, 64 hex chars, or base64", ErrInvalidMasterKey)
}

func envelopePath(dataDir, provider, keyID string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", errors.New("crypto: KMS envelope providers require --data-dir")
	}
	sum := sha256.Sum256([]byte(provider + "\x00" + keyID))
	name := fmt.Sprintf(".entdb-master-key.%s.%s.blob", provider, hex.EncodeToString(sum[:8]))
	return filepath.Join(dataDir, name), nil
}

func writeMasterKeyEnvelope(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create envelope dir %q: %w", dir, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return nil
}

func loadAWSMasterKey(ctx context.Context, cfg MasterKeyConfig) ([]byte, error) {
	keyID := strings.TrimSpace(cfg.KeyID)
	if keyID == "" {
		return nil, errors.New("crypto: aws kms provider requires --kms-key-id")
	}
	path, err := envelopePath(cfg.DataDir, "aws", keyID)
	if err != nil {
		return nil, err
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("crypto: load AWS config for KMS: %w", err)
	}
	client := awskms.NewFromConfig(awsCfg)
	if ciphertext, err := os.ReadFile(path); err == nil {
		out, err := client.Decrypt(ctx, &awskms.DecryptInput{CiphertextBlob: ciphertext})
		if err != nil {
			return nil, fmt.Errorf("crypto: AWS KMS decrypt master key envelope: %w", err)
		}
		return validatePlaintextMasterKey(out.Plaintext)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("crypto: read AWS master key envelope %q: %w", path, err)
	}
	out, err := client.GenerateDataKey(ctx, &awskms.GenerateDataKeyInput{
		KeyId:   aws.String(keyID),
		KeySpec: awskmstypes.DataKeySpecAes256,
	})
	if err != nil {
		return nil, fmt.Errorf("crypto: AWS KMS generate data key: %w", err)
	}
	key, err := validatePlaintextMasterKey(out.Plaintext)
	if err != nil {
		return nil, err
	}
	if err := writeMasterKeyEnvelope(path, out.CiphertextBlob); err != nil {
		return nil, fmt.Errorf("crypto: write AWS master key envelope %q: %w", path, err)
	}
	return key, nil
}

func validatePlaintextMasterKey(key []byte) ([]byte, error) {
	if len(key) != KeyLength {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidMasterKey, len(key))
	}
	return append([]byte(nil), key...), nil
}

type vaultDataKeyResponse struct {
	Data struct {
		Plaintext  string `json:"plaintext"`
		Ciphertext string `json:"ciphertext"`
	} `json:"data"`
	Errors []string `json:"errors"`
}

type vaultDecryptResponse struct {
	Data struct {
		Plaintext string `json:"plaintext"`
	} `json:"data"`
	Errors []string `json:"errors"`
}

func loadVaultMasterKey(ctx context.Context, cfg MasterKeyConfig) ([]byte, error) {
	keyID := strings.Trim(strings.TrimSpace(cfg.KeyID), "/")
	if keyID == "" {
		return nil, errors.New("crypto: vault kms provider requires --kms-key-id transit key name")
	}
	path, err := envelopePath(cfg.DataDir, "vault", keyID)
	if err != nil {
		return nil, err
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if ciphertext, err := os.ReadFile(path); err == nil {
		return vaultDecrypt(ctx, client, keyID, strings.TrimSpace(string(ciphertext)))
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("crypto: read Vault master key envelope %q: %w", path, err)
	}
	key, ciphertext, err := vaultGenerateDataKey(ctx, client, keyID)
	if err != nil {
		return nil, err
	}
	if err := writeMasterKeyEnvelope(path, []byte(ciphertext)); err != nil {
		return nil, fmt.Errorf("crypto: write Vault master key envelope %q: %w", path, err)
	}
	return key, nil
}

func vaultGenerateDataKey(ctx context.Context, client *http.Client, keyID string) ([]byte, string, error) {
	var out vaultDataKeyResponse
	if err := vaultDo(ctx, client, "transit/datakey/plaintext/"+url.PathEscape(keyID), map[string]any{"bits": 256}, &out); err != nil {
		return nil, "", err
	}
	if len(out.Errors) > 0 {
		return nil, "", fmt.Errorf("crypto: Vault datakey error: %s", strings.Join(out.Errors, "; "))
	}
	raw, err := base64.StdEncoding.DecodeString(out.Data.Plaintext)
	if err != nil {
		return nil, "", fmt.Errorf("crypto: decode Vault plaintext data key: %w", err)
	}
	key, err := validatePlaintextMasterKey(raw)
	if err != nil {
		return nil, "", err
	}
	if out.Data.Ciphertext == "" {
		return nil, "", errors.New("crypto: Vault datakey response omitted ciphertext")
	}
	return key, out.Data.Ciphertext, nil
}

func vaultDecrypt(ctx context.Context, client *http.Client, keyID, ciphertext string) ([]byte, error) {
	var out vaultDecryptResponse
	if err := vaultDo(ctx, client, "transit/decrypt/"+url.PathEscape(keyID), map[string]any{"ciphertext": ciphertext}, &out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("crypto: Vault decrypt error: %s", strings.Join(out.Errors, "; "))
	}
	raw, err := base64.StdEncoding.DecodeString(out.Data.Plaintext)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode Vault plaintext master key: %w", err)
	}
	return validatePlaintextMasterKey(raw)
}

func vaultDo(ctx context.Context, client *http.Client, path string, payload map[string]any, out any) error {
	addr := strings.TrimRight(strings.TrimSpace(os.Getenv("VAULT_ADDR")), "/")
	if addr == "" {
		return errors.New("crypto: VAULT_ADDR is required for vault kms provider")
	}
	token := strings.TrimSpace(os.Getenv("VAULT_TOKEN"))
	if token == "" {
		return errors.New("crypto: VAULT_TOKEN is required for vault kms provider")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, addr+"/v1/"+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("crypto: build Vault request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vault-Token", token)
	if ns := strings.TrimSpace(os.Getenv("VAULT_NAMESPACE")); ns != "" {
		req.Header.Set("X-Vault-Namespace", ns)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("crypto: Vault request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("crypto: read Vault response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("crypto: Vault request returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("crypto: parse Vault response: %w", err)
	}
	return nil
}
