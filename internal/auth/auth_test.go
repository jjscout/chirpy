package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashAndPassword(t *testing.T) {
	password := "my-secret-password"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error hashing password: %v", err)
	}

	if hash == password {
		t.Errorf("expected hash to be different from the password")
	}

	if len(hash) == 0 {
		t.Errorf("expected non-empty hash")
	}

	// Verify correct password matches
	match, err := CheckPasswordHash(password, hash)
	if err != nil {
		t.Fatalf("unexpected error checking password hash: %v", err)
	}
	if !match {
		t.Errorf("expected password to match its hash")
	}

	// Verify incorrect password does not match
	match, err = CheckPasswordHash("wrong-password", hash)
	if err != nil {
		t.Fatalf("unexpected error checking password hash with wrong password: %v", err)
	}
	if match {
		t.Errorf("expected wrong password not to match the hash")
	}
}

func TestJWT(t *testing.T) {
	tuuid := uuid.New()
	secret := "123456"
	expiresIn, err := time.ParseDuration("1000ms")
	token, err := MakeJWT(tuuid, secret, expiresIn)
	if err != nil {
		t.Fatalf("unexpected error checking jwt: %v", err)
	}
	puuid, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("unexpected error checking jwt: %v", err)
	}
	if puuid == uuid.Nil {
		t.Fatalf("wrong token")

	}
	time.Sleep(expiresIn)
	puuid, err = ValidateJWT(token, secret)
	if err == nil {
		t.Fatalf("jwt should have expired")
	}

}
