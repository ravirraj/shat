package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	if kp.PrivateKey == nil {
		t.Fatal("private key is nil")
	}
	if kp.PublicKey == nil {
		t.Fatal("public key is nil")
	}
}

func TestMarshalParsePublicKey(t *testing.T) {
	kp, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	pubBytes, err := kp.MarshalPublicKey()
	if err != nil {
		t.Fatalf("MarshalPublicKey failed: %v", err)
	}

	parsed, err := ParsePublicKey(pubBytes)
	if err != nil {
		t.Fatalf("ParsePublicKey failed: %v", err)
	}

	if parsed.N.Cmp(kp.PublicKey.N) != 0 {
		t.Fatal("parsed public key does not match original")
	}
}

func TestMarshalParsePrivateKey(t *testing.T) {
	kp, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	privBytes, err := kp.MarshalPrivateKey()
	if err != nil {
		t.Fatalf("MarshalPrivateKey failed: %v", err)
	}

	parsed, err := ParsePrivateKey(privBytes)
	if err != nil {
		t.Fatalf("ParsePrivateKey failed: %v", err)
	}

	if parsed.D.Cmp(kp.PrivateKey.D) != 0 {
		t.Fatal("parsed private key does not match original")
	}
}

func TestEncryptDecryptAES(t *testing.T) {
	key, err := GenerateAESKey()
	if err != nil {
		t.Fatalf("GenerateAESKey failed: %v", err)
	}

	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"hello", []byte("hello world")},
		{"binary", []byte{0x00, 0x01, 0xFF, 0xFE, 0x80}},
		{"large", bytes.Repeat([]byte("x"), 10000)},
		{"unicode", []byte("Hello, 世界! 🌍")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := EncryptAES(key, tt.plaintext)
			if err != nil {
				t.Fatalf("EncryptAES failed: %v", err)
			}

			plaintext, err := DecryptAES(key, ciphertext)
			if err != nil {
				t.Fatalf("DecryptAES failed: %v", err)
			}

			if !bytes.Equal(plaintext, tt.plaintext) {
				t.Errorf("decrypted text does not match original: got %q, want %q", plaintext, tt.plaintext)
			}
		})
	}
}

func TestEncryptDecryptAESWrongKey(t *testing.T) {
	key1, _ := GenerateAESKey()
	key2, _ := GenerateAESKey()

	ciphertext, err := EncryptAES(key1, []byte("secret"))
	if err != nil {
		t.Fatalf("EncryptAES failed: %v", err)
	}

	_, err = DecryptAES(key2, ciphertext)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestEncryptDecryptRSA(t *testing.T) {
	kp, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	data := []byte("test data for RSA encryption")
	ciphertext, err := EncryptWithRSA(kp.PublicKey, data)
	if err != nil {
		t.Fatalf("EncryptWithRSA failed: %v", err)
	}

	plaintext, err := DecryptWithRSA(kp.PrivateKey, ciphertext)
	if err != nil {
		t.Fatalf("DecryptWithRSA failed: %v", err)
	}

	if !bytes.Equal(plaintext, data) {
		t.Errorf("decrypted text does not match: got %q, want %q", plaintext, data)
	}
}

func TestEncryptWithRSALargeData(t *testing.T) {
	kp, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	data := make([]byte, 200)
	rand.Read(data)

	_, err = EncryptWithRSA(kp.PublicKey, data)
	if err == nil {
		t.Fatal("expected error for data too large for RSA")
	}
}

func BenchmarkEncryptAES(b *testing.B) {
	key, _ := GenerateAESKey()
	data := bytes.Repeat([]byte("x"), 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncryptAES(key, data)
	}
}

func BenchmarkDecryptAES(b *testing.B) {
	key, _ := GenerateAESKey()
	data := bytes.Repeat([]byte("x"), 1024)
	ciphertext, _ := EncryptAES(key, data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DecryptAES(key, ciphertext)
	}
}
