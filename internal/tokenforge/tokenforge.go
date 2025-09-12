package tokenforge

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	tokenVersion  byte   = 1
	defaultTime   uint32 = 4
	defaultMemory uint32 = 64 * 1024 // 64 MB
	kdfSaltLength        = 64
)

func MakeToken(secret string, ttl time.Duration) (string, error) {
	uniq := make([]byte, 32)
	var err error
	var bytesRead int
	if bytesRead, err = rand.Read(uniq); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}

	if bytesRead != 32 {
		return "", fmt.Errorf("rand: short read")
	}

	exp := time.Now().Add(ttl).Unix()
	expBytes := make([]byte, 8)
	//nolint:gosec // how could int64 -> uint64 be an overflow?
	binary.BigEndian.PutUint64(expBytes, uint64(exp))

	// Generate random KDF salt for this token
	kdfSalt := make([]byte, kdfSaltLength)
	if bytesRead, err = rand.Read(kdfSalt); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	if bytesRead != kdfSaltLength {
		return "", fmt.Errorf("rand: short read")
	}

	// Build header
	buf := []byte{tokenVersion}
	buf = append(buf, uniq...)
	buf = append(buf, expBytes...)

	timeBytes := make([]byte, 4)
	memBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(timeBytes, defaultTime)
	binary.BigEndian.PutUint32(memBytes, defaultMemory)

	numCPU := math.Max(1, float64(runtime.NumCPU()))
	if numCPU > 255 {
		numCPU = 255
	}
	parallel := uint8(numCPU)

	buf = append(buf, timeBytes...)
	buf = append(buf, memBytes...)
	buf = append(buf, parallel)
	buf = append(buf, kdfSalt...)

	// Derive key
	key := argon2.IDKey([]byte(secret), kdfSalt, defaultTime, defaultMemory, parallel, 64)

	// Sign (HMAC over everything so far)
	h := hmac.New(sha512.New, key)
	bytesWritten, err := h.Write(buf)
	if err != nil {
		return "", fmt.Errorf("hmac: %w", err)
	}
	if bytesWritten != len(buf) {
		return "", fmt.Errorf("hmac: short write")
	}
	sig := h.Sum(nil)

	buf = append(buf, sig...)
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func VerifyToken(secret, token string) (bool, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false, fmt.Errorf("decode: %w", err)
	}
	minLen := 1 + 32 + 8 + 4 + 4 + 1 + kdfSaltLength + 64
	if len(decoded) < minLen {
		return false, fmt.Errorf("too short")
	}

	ver := decoded[0]
	if ver != tokenVersion {
		return false, fmt.Errorf("unsupported version %d", ver)
	}

	// uniq := decoded[1:33]
	expBytes := decoded[33:41]
	timeCost := binary.BigEndian.Uint32(decoded[41:45])
	memCost := binary.BigEndian.Uint32(decoded[45:49])
	parallelism := decoded[49]
	kdfSalt := decoded[50 : 50+kdfSaltLength]
	rxSig := decoded[50+kdfSaltLength:]

	expUint := binary.BigEndian.Uint64(expBytes)
	if expUint > math.MaxInt64 {
		return false, fmt.Errorf("invalid expiration")
	}
	exp := int64(expUint)
	if time.Now().Unix() > exp {
		return false, fmt.Errorf("expired")
	}

	// Re-derive key using the params and salt inside the token
	key := argon2.IDKey([]byte(secret), kdfSalt, timeCost, memCost, parallelism, 64)

	// Verify signature
	msg := decoded[:50+kdfSaltLength] // everything up to salt
	h := hmac.New(sha512.New, key)
	bytesWritten, err := h.Write(msg)
	if err != nil {
		return false, fmt.Errorf("hmac: %w", err)
	}
	if bytesWritten != len(msg) {
		return false, fmt.Errorf("hmac: short write")
	}

	expected := h.Sum(nil)

	if !hmac.Equal(expected, rxSig) {
		return false, fmt.Errorf("bad signature")
	}

	return true, nil
}
