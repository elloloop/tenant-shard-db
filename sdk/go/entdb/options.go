package entdb

import "time"

// clientConfig holds all client configuration values.
type clientConfig struct {
	secure     bool
	apiKey     string
	maxRetries int
	timeout    time.Duration
}

// defaultConfig returns a clientConfig with sensible defaults.
func defaultConfig() clientConfig {
	return clientConfig{
		secure:     false,
		apiKey:     "",
		maxRetries: 3,
		timeout:    30 * time.Second,
	}
}

// ClientOption is a functional option for configuring DbClient.
type ClientOption func(*clientConfig)

// WithSecure enables TLS for the gRPC connection.
func WithSecure() ClientOption {
	return func(c *clientConfig) {
		c.secure = true
	}
}

// WithAPIKey sets the API key used for authentication.
func WithAPIKey(key string) ClientOption {
	return func(c *clientConfig) {
		c.apiKey = key
	}
}

// WithMaxRetries sets the maximum number of retry attempts for failed RPCs.
func WithMaxRetries(n int) ClientOption {
	return func(c *clientConfig) {
		if n >= 0 {
			c.maxRetries = n
		}
	}
}

// WithTimeout sets the default timeout for RPCs.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *clientConfig) {
		c.timeout = d
	}
}
