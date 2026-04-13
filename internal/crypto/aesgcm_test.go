package crypto

import (
	"bytes"
	"testing"
)

func TestAESEncryptorRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := NewAESEncryptor(key)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("xoxb-test-token")
	ct, err := enc.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ct, plain) {
		t.Fatal("ciphertext equals plaintext")
	}
	out, err := enc.Decrypt(ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("got %q want %q", out, plain)
	}
}

func TestParseKeyBase64(t *testing.T) {
	// echo -n '0123456789abcdef0123456789abcdef' | base64
	const b64 = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	key, err := ParseKeyBase64(b64)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 {
		t.Fatalf("len %d", len(key))
	}
}
