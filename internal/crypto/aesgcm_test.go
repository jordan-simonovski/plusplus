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
	ct, err := enc.Encrypt(plain, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ct, plain) {
		t.Fatal("ciphertext equals plaintext")
	}
	out, err := enc.Decrypt(ct, nil)
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

func TestAESEncryptorBindsAAD(t *testing.T) {
	enc, err := NewAESEncryptor(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := enc.Encrypt([]byte("xoxb-team-a"), []byte("TA"))
	if err != nil {
		t.Fatal(err)
	}
	if out, err := enc.Decrypt(ct, []byte("TA")); err != nil || string(out) != "xoxb-team-a" {
		t.Fatalf("same AAD: %q, %v", out, err)
	}
	if _, err := enc.Decrypt(ct, []byte("TB")); err == nil {
		t.Fatal("decrypting with another team's AAD must fail")
	}
	if _, err := enc.Decrypt(ct, nil); err == nil {
		t.Fatal("decrypting without the AAD must fail")
	}
}
