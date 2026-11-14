package crypto

import (
	"bytes"
	"encoding/base64"
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

func TestAESEncryptorRejectsBadKeys(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33} {
		if _, err := NewAESEncryptor(make([]byte, n)); err == nil {
			t.Errorf("NewAESEncryptor with %d-byte key: want an error", n)
		}
	}
	for name, s := range map[string]string{
		"empty":      "",
		"bad base64": "not base64!!",
		"16 bytes":   base64.StdEncoding.EncodeToString(make([]byte, 16)),
	} {
		if _, err := ParseKeyBase64(s); err == nil {
			t.Errorf("ParseKeyBase64 %s: want an error", name)
		}
	}
}

func TestAESEncryptorDecryptRejectsDamagedBlobs(t *testing.T) {
	enc, err := NewAESEncryptor(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := enc.Encrypt([]byte("xoxb-token"), []byte("T1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enc.Decrypt(ct[:5], []byte("T1")); err == nil {
		t.Error("blob shorter than the nonce: want an error")
	}
	flipped := append([]byte(nil), ct...)
	flipped[len(flipped)/2] ^= 0x01
	if _, err := enc.Decrypt(flipped, []byte("T1")); err == nil {
		t.Error("blob with a flipped bit: want an error")
	}
	again, _ := enc.Encrypt([]byte("xoxb-token"), []byte("T1"))
	if bytes.Equal(ct, again) {
		t.Error("two encryptions of the same plaintext are identical; the nonce must be random")
	}
}
