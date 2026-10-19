//go:build linux

package container

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cardinal/internal/state"
)

const (
	// EncryptionKeyEnv is the environment variable for the encryption key.
	EncryptionKeyEnv = "CARDINAL_BACKUP_KEY"

	// EncryptionKeyFile is the path to the encryption key file.
	EncryptionKeyFile = ".backup-key"

	// GCMNonceSize is the size of the GCM nonce.
	GCMNonceSize = 12

	// GCMTagSize is the size of the GCM authentication tag.
	GCMTagSize = 16

	// backupFormatV2 — маркер нового формата шифрования:
	// соль + nonce + ciphertext. Старый формат (nonce + ciphertext, ключ
	// из одного SHA-256) читается по-прежнему.
	backupFormatV2 = "CARDBAK2"

	// backupSaltSize — размер соли для вывода ключа (KDF).
	backupSaltSize = 16

	// pbkdf2Iterations — число итераций PBKDF2-HMAC-SHA256 (OWASP: 600k).
	pbkdf2Iterations = 600000
)

// BackupEncryptor provides encryption/decryption for backup files.
type BackupEncryptor struct {
	// raw — исходный материал ключа (как задал оператор). Реальный ключ
	// AES выводится из него через KDF с солью, соль хранится в файле.
	raw []byte
}

// NewBackupEncryptor creates a new BackupEncryptor with the given key.
//
// Реальный ключ AES выводится из материала через PBKDF2-HMAC-SHA256 со
// случайной солью (соль — в заголовке файла). Раньше не-32-байтовый ключ
// кодировался ОДНИМ SHA-256: GCM аутентифицирован, но стойкость ключа
// определялась KDF, а перебор паролей шёл на полной скорости SHA-256.
func NewBackupEncryptor(key []byte) (*BackupEncryptor, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("empty encryption key")
	}
	return &BackupEncryptor{raw: key}, nil
}

// derive выводит ключ AES из материала с заданной солью.
func (e *BackupEncryptor) derive(salt []byte) ([]byte, error) {
	return pbkdf2.Key(sha256.New, string(e.raw), salt, pbkdf2Iterations, 32)
}

// legacyKey воспроизводит старую схему вывода ключа (один SHA-256),
// чтобы старые зашифрованные бэкапы оставались читаемыми.
func (e *BackupEncryptor) legacyKey() []byte {
	if len(e.raw) == 32 {
		return e.raw
	}
	h := sha256.Sum256(e.raw)
	return h[:]
}

// NewBackupEncryptorFromEnv creates a BackupEncryptor from the CARDINAL_BACKUP_KEY env var.
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
	// Ключ лежит в каталоге состояния (0700), а не в текущем рабочем
	// каталоге: раньше путь был относительным, и файл ключа оказывался
	// там, откуда запустили cardinal (репозиторий, общая папка, образ
	// контейнера) — то есть ключ уезжал в бэкапы и архивы.
	return filepath.Join(state.DataDir(), EncryptionKeyFile)
}

// Encrypt encrypts the input data using AES-256-GCM.
//
// Формат: "CARDBAK2" || salt(16) || nonce(12) || ciphertext+tag.
// Старый формат (nonce || ciphertext) тоже читается — см. Decrypt.
func (e *BackupEncryptor) Encrypt(data []byte) ([]byte, error) {
	salt := make([]byte, backupSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	key, err := e.derive(salt)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	block, err := aes.NewCipher(key)
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

	ciphertext := gcm.Seal(nil, nonce, data, nil)
	out := make([]byte, 0, len(backupFormatV2)+len(salt)+len(nonce)+len(ciphertext))
	out = append(out, backupFormatV2...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// Decrypt decrypts the input data using AES-256-GCM.
//
// Понимает оба формата: новый (с солью) и старый (ключ из одного SHA-256).
func (e *BackupEncryptor) Decrypt(data []byte) ([]byte, error) {
	if len(data) >= len(backupFormatV2) && string(data[:len(backupFormatV2)]) == backupFormatV2 {
		rest := data[len(backupFormatV2):]
		if len(rest) < backupSaltSize+GCMNonceSize {
			return nil, fmt.Errorf("ciphertext too short")
		}
		salt := rest[:backupSaltSize]
		key, err := e.derive(salt)
		if err != nil {
			return nil, fmt.Errorf("derive key: %w", err)
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("create cipher: %w", err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("create GCM: %w", err)
		}
		nonce := rest[backupSaltSize : backupSaltSize+GCMNonceSize]
		ciphertext := rest[backupSaltSize+GCMNonceSize:]
		return gcm.Open(nil, nonce, ciphertext, nil)
	}

	// Старый формат: nonce || ciphertext, ключ — один SHA-256.
	key := e.legacyKey()
	block, err := aes.NewCipher(key)
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
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cardinal-enc-*.tmp")
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
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cardinal-dec-*.tmp")
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
	defer func() { _ = f.Close() }()

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
