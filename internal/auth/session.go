// Package auth authenticates API requests using better-auth sessions.
//
// better-auth (running in the Next.js app) owns sign-up, sign-in and session
// storage. It sets a signed cookie whose value is
// url-encode("<token>.<base64 HMAC-SHA256(secret, token)>"). The Go API
// verifies that signature with the shared BETTER_AUTH_SECRET and then looks
// the token up in the "session" table.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/codebyaadi/feedoku/internal/store"
)

// Cookie names used by better-auth. The __Secure- variant is used when the
// app is served over HTTPS.
var sessionCookieNames = []string{
	"__Secure-better-auth.session_token",
	"better-auth.session_token",
}

var (
	ErrNoSession       = errors.New("no session cookie")
	ErrInvalidSession  = errors.New("invalid session")
	errBadSignature    = errors.New("bad cookie signature")
	errMalformedCookie = errors.New("malformed session cookie")
)

type User struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"emailVerified"`
	Image         *string   `json:"image"`
	CreatedAt     time.Time `json:"createdAt"`
}

type SessionQuerier interface {
	GetSessionUser(ctx context.Context, token string) (store.GetSessionUserRow, error)
}

type Authenticator struct {
	secret []byte
	db     SessionQuerier
}

func NewAuthenticator(secret string, db SessionQuerier) *Authenticator {
	return &Authenticator{secret: []byte(secret), db: db}
}

// Authenticate resolves the user behind the request's session cookie.
func (a *Authenticator) Authenticate(r *http.Request) (User, error) {
	raw, ok := sessionCookie(r)
	if !ok {
		return User{}, ErrNoSession
	}

	token, err := VerifySignedValue(raw, a.secret)
	if err != nil {
		return User{}, ErrInvalidSession
	}

	row, err := a.db.GetSessionUser(r.Context(), token)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidSession
	}
	if err != nil {
		return User{}, err
	}

	return User{
		ID:            row.ID,
		Name:          row.Name,
		Email:         row.Email,
		EmailVerified: row.EmailVerified,
		Image:         row.Image,
		CreatedAt:     row.CreatedAt.UTC(),
	}, nil
}

func sessionCookie(r *http.Request) (string, bool) {
	for _, name := range sessionCookieNames {
		if c, err := r.Cookie(name); err == nil && c.Value != "" {
			return c.Value, true
		}
	}
	return "", false
}

// VerifySignedValue checks a better-call signed cookie value and returns the
// unsigned token.
func VerifySignedValue(raw string, secret []byte) (string, error) {
	value, err := url.PathUnescape(raw)
	if err != nil {
		return "", errMalformedCookie
	}

	i := strings.LastIndexByte(value, '.')
	if i < 1 {
		return "", errMalformedCookie
	}
	token, sig := value[:i], value[i+1:]

	got, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return "", errMalformedCookie
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(token))
	if !hmac.Equal(got, mac.Sum(nil)) {
		return "", errBadSignature
	}
	return token, nil
}

// SignValue produces a cookie value in better-call's format. Used in tests.
func SignValue(token string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(token))
	return url.QueryEscape(token + "." + base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}
