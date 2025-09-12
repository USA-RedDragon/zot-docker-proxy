package tokenforge

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

const testSecret = "supersecretkey"

func TestMakeTokenAndVerifyToken_Success(t *testing.T) {
	t.Parallel()
	ttl := 2 * time.Minute
	token, err := MakeToken(testSecret, ttl)
	if err != nil {
		t.Fatalf("MakeToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}
	ok, err := VerifyToken(testSecret, token)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if !ok {
		t.Fatal("VerifyToken should return true for valid token")
	}
}

func TestVerifyToken_BadSecret(t *testing.T) {
	t.Parallel()
	secret := "supersecretkey"
	badSecret := "wrongsecret"
	ttl := 1 * time.Minute
	token, err := MakeToken(secret, ttl)
	if err != nil {
		t.Fatalf("MakeToken failed: %v", err)
	}
	ok, err := VerifyToken(badSecret, token)
	if err == nil || ok {
		t.Fatal("VerifyToken should fail with wrong secret")
	}
	if !strings.Contains(err.Error(), "bad signature") {
		t.Errorf("expected bad signature error, got: %v", err)
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	t.Parallel()
	ttl := -1 * time.Second // already expired
	token, err := MakeToken(testSecret, ttl)
	if err != nil {
		t.Fatalf("MakeToken failed: %v", err)
	}
	ok, err := VerifyToken(testSecret, token)
	if err == nil || ok {
		t.Fatal("VerifyToken should fail for expired token")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected expired error, got: %v", err)
	}
}

func TestVerifyToken_InvalidBase64(t *testing.T) {
	t.Parallel()
	token := "not_base64!!"
	ok, err := VerifyToken(testSecret, token)
	if err == nil || ok {
		t.Fatal("VerifyToken should fail for invalid base64 token")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected decode error, got: %v", err)
	}
}

func TestVerifyToken_TooShort(t *testing.T) {
	t.Parallel()
	// Make a valid base64 string but too short
	short := base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3})
	ok, err := VerifyToken(testSecret, short)
	if err == nil || ok {
		t.Fatal("VerifyToken should fail for too short token")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Errorf("expected too short error, got: %v", err)
	}
}
