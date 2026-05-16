package crypto

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	sqlcipher "github.com/mutecomm/go-sqlcipher/v4"
)

const (
	// SQLCipherDriverName is the database/sql driver registered by
	// github.com/mutecomm/go-sqlcipher/v4.
	SQLCipherDriverName = "sqlite3"

	sqlcipherPageSize = 4096
)

// SQLCipherDSN returns a database/sql DSN that opens path with key as a
// raw 256-bit SQLCipher key.
func SQLCipherDSN(path string, key []byte, busyTimeout time.Duration) (string, error) {
	if len(key) != KeyLength {
		return "", fmt.Errorf("%w: got %d", ErrInvalidMasterKey, len(key))
	}
	if busyTimeout <= 0 {
		busyTimeout = 5 * time.Second
	}
	return fmt.Sprintf(
		"file:%s?_pragma_key=x'%s'&_pragma_cipher_page_size=%d&_busy_timeout=%d",
		path,
		hex.EncodeToString(key),
		sqlcipherPageSize,
		busyTimeout.Milliseconds(),
	), nil
}

// SQLiteFileEncryptionStatus reports whether path exists and, when it
// already holds a complete SQLite header, whether SQLCipher encrypted it.
func SQLiteFileEncryptionStatus(path string) (encrypted bool, hasDatabase bool, err error) {
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("crypto: stat sqlite file %q: %w", path, err)
	}
	if st.Size() == 0 {
		return false, false, nil
	}
	encrypted, err = sqlcipher.IsEncrypted(path)
	if err != nil {
		return false, true, fmt.Errorf("crypto: inspect sqlite encryption %q: %w", path, err)
	}
	return encrypted, true, nil
}
