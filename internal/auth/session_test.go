package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/codebyaadi/feedoku/internal/store"
)

const testSecret = "test-secret-please-change-0123456789"

// Produced by better-call's signCookieValue (the library better-auth uses)
// for token "Z3vKq8xYtR2mN5pL0aBcDeFgHiJkLmNo" and testSecret.
const betterCallVector = "Z3vKq8xYtR2mN5pL0aBcDeFgHiJkLmNo.JxJG3MH19hMf0eSXBf5ujEwZ1mC8ra61JMQupfUuqkg%3D"

func TestVerifySignedValue(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		secret  string
		want    string
		wantErr bool
	}{
		{name: "better-call vector", raw: betterCallVector, secret: testSecret, want: "Z3vKq8xYtR2mN5pL0aBcDeFgHiJkLmNo"},
		{name: "round trip", raw: SignValue("abc.def", []byte(testSecret)), secret: testSecret, want: "abc.def"},
		{name: "wrong secret", raw: betterCallVector, secret: "other", wantErr: true},
		{name: "tampered token", raw: "X" + betterCallVector[1:], secret: testSecret, wantErr: true},
		{name: "unsigned", raw: "Z3vKq8xYtR2mN5pL0aBcDeFgHiJkLmNo", secret: testSecret, wantErr: true},
		{name: "bad base64", raw: "token.%%%", secret: testSecret, wantErr: true},
		{name: "empty", raw: "", secret: testSecret, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifySignedValue(tt.raw, []byte(tt.secret))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

type fakeSessions map[string]store.GetSessionUserRow

func (f fakeSessions) GetSessionUser(_ context.Context, token string) (store.GetSessionUserRow, error) {
	row, ok := f[token]
	if !ok {
		return row, pgx.ErrNoRows
	}
	return row, nil
}

func TestAuthenticate(t *testing.T) {
	db := fakeSessions{"good-token": {ID: "u1", Email: "a@example.com", SessionExpiresAt: time.Now().Add(time.Hour)}}
	a := NewAuthenticator(testSecret, db)

	request := func(name, value string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if name != "" {
			r.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		return r
	}

	if u, err := a.Authenticate(request("better-auth.session_token", SignValue("good-token", []byte(testSecret)))); err != nil || u.ID != "u1" {
		t.Fatalf("valid cookie: user=%+v err=%v", u, err)
	}
	if u, err := a.Authenticate(request("__Secure-better-auth.session_token", SignValue("good-token", []byte(testSecret)))); err != nil || u.ID != "u1" {
		t.Fatalf("secure cookie: user=%+v err=%v", u, err)
	}
	if _, err := a.Authenticate(request("", "")); err != ErrNoSession {
		t.Fatalf("no cookie: err=%v", err)
	}
	if _, err := a.Authenticate(request("better-auth.session_token", SignValue("unknown", []byte(testSecret)))); err != ErrInvalidSession {
		t.Fatalf("unknown token: err=%v", err)
	}
	if _, err := a.Authenticate(request("better-auth.session_token", SignValue("good-token", []byte("wrong")))); err != ErrInvalidSession {
		t.Fatalf("forged cookie: err=%v", err)
	}
}
