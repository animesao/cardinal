//go:build linux

package container

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

// TestBackupEncryptRoundTrip — шифрование/дешифрование работает в обе
// стороны, и ключ выводится через KDF (PBKDF2), а не одним SHA-256.
func TestBackupEncryptRoundTrip(t *testing.T) {
	enc, err := NewBackupEncryptor([]byte("correct horse battery staple"))
	if err != nil {
		t.Fatalf("NewBackupEncryptor: %v", err)
	}
	plain := []byte("данные арендатора: файлы сервера")
	ct, err := enc.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Contains(ct, plain) {
		t.Fatal("шифротекст содержит открытый текст")
	}
	if !bytes.HasPrefix(ct, []byte(backupFormatV2)) {
		t.Fatal("нет маркера нового формата")
	}
	out, err := enc.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("расшифровано не то: %q", out)
	}
}

// TestBackupEncryptSaltIsRandom — соль случайна: два шифрования одних и
// тех же данных дают разный шифротекст (иначе одинаковые бэкапы
// сравнимы).
func TestBackupEncryptSaltIsRandom(t *testing.T) {
	enc, err := NewBackupEncryptor([]byte("passphrase"))
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("same data")
	a, err := enc.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	b, err := enc.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("шифротексты совпали: соль не случайна")
	}
}

// TestBackupDecryptLegacyFormat — старые зашифрованные бэкапы (ключ из
// одного SHA-256, формат nonce||ciphertext) остаются читаемыми.
func TestBackupDecryptLegacyFormat(t *testing.T) {
	raw := []byte("legacy-passphrase")
	sum := sha256.Sum256(raw)
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	copy(nonce, []byte("123456789012"))
	plain := []byte("old backup payload")
	legacy := gcm.Seal(append([]byte{}, nonce...), nonce, plain, nil)

	enc, err := NewBackupEncryptor(raw)
	if err != nil {
		t.Fatal(err)
	}
	out, err := enc.Decrypt(legacy)
	if err != nil {
		t.Fatalf("Decrypt legacy: %v", err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("legacy расшифровано не то: %q", out)
	}
}

// TestBackupWrongKeyFails — чужой ключ не расшифровывает (GCM-тег).
func TestBackupWrongKeyFails(t *testing.T) {
	enc, err := NewBackupEncryptor([]byte("key-one"))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := enc.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewBackupEncryptor([]byte("key-two"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Decrypt(ct); err == nil {
		t.Fatal("чужой ключ расшифровал данные")
	}
}

// TestBackupKeyFilePathInStateDir — файл ключа лежит в каталоге состояния,
// а не в текущем рабочем каталоге.
func TestBackupKeyFilePathInStateDir(t *testing.T) {
	p := getKeyFilePath()
	if !filepath.IsAbs(p) {
		t.Fatalf("путь к файлу ключа относительный: %q", p)
	}
	if filepath.Base(p) != EncryptionKeyFile {
		t.Fatalf("неожиданное имя файла ключа: %q", p)
	}
}

// TestSaveEncryptionKeyPermissions — ключ пишется с правами 0600.
func TestSaveEncryptionKeyPermissions(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, EncryptionKeyFile)
	if err := os.WriteFile(keyPath, []byte("00"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("файл ключа доступен группе/остальным: %v", info.Mode().Perm())
	}
}
