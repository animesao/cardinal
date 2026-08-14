//go:build linux

package container

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// EncryptionKeyEnv is the environment variable for the encryption key.
	EncryptionKeyEnv = "DCK_BACKUP_KEY"

	// EncryptionKeyFile is the path to the encryption key file.
	EncryptionKeyFile = ".backup-key"

	// GCMNonceSize is the size of the GCM nonce.
	GCMNonceSize = 12

	// GCMTagSize is the size of the GCM authentication tag.
	GCMTagSize = 16
)

// BackupEncryptor provides encryption/decryption for backup files.
type BackupEncryptor struct {
	key []byte
}

// NewBackupEncryptor creates a new BackupEncryptor with the given key.
func NewBackupEncryptor(key []byte) (*BackupEncryptor, error) {
	if len(key) != 32 { // AES-256
		// Derive a 32-byte key from the provided key using SHA-256
		hash := sha256.Sum256(key)
		key = hash[:]
	}

	return &BackupEncryptor{key: key}, nil
}

// NewBackupEncryptorFromEnv creates a BackupEncryptor from the DCK_BACKUP_KEY env var.
func NewBackupEncryptorFromEnv() (*BackupEncryptor, error) {
	keyStr := os.Getenv(EncryptionKeyEnv)
	if keyStr == "" {
		// Try to load from file
		keyPath := getKeyFilePath()
		if data, err := os.ReadFile(keyPath); err == nil {
			keyStr = strings.TrimSpace(string(data))
		}
	}

	if keyStr == "" {
		return nil, fmt.Errorf("encryption key not found (set %s env var or create %s file)", EncryptionKeyEnv, EncryptionKeyFile)
	}

	// Support hex-encoded keys
	key, err := hex.DecodeString(keyStr)
	if err != nil {
		// Treat as raw string key
		key = []byte(keyStr)
	}

	return NewBackupEncryptor(key)
}

// GenerateEncryptionKey generates a new random 32-byte encryption key.
func GenerateEncryptionKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate encryption key: %w", err)
	}
	return key, nil
}

// SaveEncryptionKey saves an encryption key to the default key file.
func SaveEncryptionKey(key []byte) error {
	keyPath := getKeyFilePath()
	return os.WriteFile(keyPath, []byte(hex.EncodeToString(key)), 0600)
}

func getKeyFilePath() string {
	return EncryptionKeyFile
}

// Encrypt encrypts the input data using AES-256-GCM.
func (e *BackupEncryptor) Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt and append nonce
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// Decrypt decrypts the input data using AES-256-GCM.
func (e *BackupEncryptor) Decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// EncryptFile encrypts a file in-place.
func (e *BackupEncryptor) EncryptFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	encrypted, err := e.Encrypt(data)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	// Write to temp file and rename for atomicity
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dck-enc-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(encrypted); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write encrypted: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

// DecryptFile decrypts a file in-place.
func (e *BackupEncryptor) DecryptFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	decrypted, err := e.Decrypt(data)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	// Write to temp file and rename for atomicity
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dck-dec-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(decrypted); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write decrypted: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

// EncryptReader encrypts data from a reader and writes to a writer.
func (e *BackupEncryptor) EncryptReader(r io.Reader, w io.Writer) error {
	// Read all data
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	encrypted, err := e.Encrypt(data)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	_, err = w.Write(encrypted)
	return err
}

// DecryptReader decrypts data from a reader and writes to a writer.
func (e *BackupEncryptor) DecryptReader(r io.Reader, w io.Writer) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	decrypted, err := e.Decrypt(data)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	_, err = w.Write(decrypted)
	return err
}

// IsEncrypted checks if a file appears to be encrypted.
// Encrypted files start with a GCM nonce (12 bytes) followed by ciphertext.
func IsEncrypted(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Read first 12 bytes (nonce size) + some ciphertext
	header := make([]byte, 32)
	n, err := f.Read(header)
	if err != nil || n < 32 {
		return false, nil
	}

	// Simple heuristic: if the file doesn't start with valid gzip/tar magic,
	// it might be encrypted. This is not perfect but works for common cases.
	if header[0] == 0x1f && header[1] == 0x8b { // gzip magic
		return false, nil
	}

	// Check for tar magic (ustar)
	if n >= 263 && string(header[257:263]) == "ustar" {
		return false, nil
	}

	return true, nil
}

// SecureDelete overwrites a file with random data before deleting it.
func SecureDelete(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	size := info.Size()

	// Overwrite with random data
	f, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	// Overwrite 3 times
	for i := 0; i < 3; i++ {
		if _, err := f.Seek(0, 0); err != nil {
			_ = f.Close()
			return err
		}

		buf := make([]byte, 4096)
		written := int64(0)
		for written < size {
			toWrite := int64(len(buf))
			if toWrite > size-written {
				toWrite = size - written
			}

			if _, err := rand.Read(buf[:toWrite]); err != nil {
				_ = f.Close()
				return err
			}

			if _, err := f.Write(buf[:toWrite]); err != nil {
				_ = f.Close()
				return err
			}

			written += toWrite
		}

		if err := f.Sync(); err != nil {
			_ = f.Close()
			return err
		}
	}

	_ = f.Close()
	return os.Remove(path)
}


