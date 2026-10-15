package cfaccess

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type fixture struct {
	key *rsa.PrivateKey
	v   *Verifier
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1",
			"kty": "RSA",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(srv.Close)

	v := New("team.example.com", "aud-1")
	v.certsURL = srv.URL
	return &fixture{key: key, v: v}
}

func (f *fixture) token(t *testing.T, mutate func(*jwt.RegisteredClaims), kid string, method jwt.SigningMethod, key any) string {
	t.Helper()
	rc := jwt.RegisteredClaims{
		Issuer:    "https://team.example.com",
		Audience:  jwt.ClaimStrings{"aud-1"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	if mutate != nil {
		mutate(&rc)
	}
	tok := jwt.NewWithClaims(method, Claims{Email: "admin@example.com", RegisteredClaims: rc})
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerifyAcceptsValidToken(t *testing.T) {
	f := newFixture(t)
	c, err := f.v.Verify(context.Background(), f.token(t, nil, "k1", jwt.SigningMethodRS256, f.key))
	if err != nil {
		t.Fatal(err)
	}
	if c.Email != "admin@example.com" {
		t.Fatalf("email = %q", c.Email)
	}
}

func TestVerifyRejects(t *testing.T) {
	f := newFixture(t)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)

	cases := map[string]string{
		"빈 토큰":       "",
		"다른 AUD":     f.token(t, func(c *jwt.RegisteredClaims) { c.Audience = jwt.ClaimStrings{"other"} }, "k1", jwt.SigningMethodRS256, f.key),
		"다른 발급자":     f.token(t, func(c *jwt.RegisteredClaims) { c.Issuer = "https://evil.example.com" }, "k1", jwt.SigningMethodRS256, f.key),
		"만료":         f.token(t, func(c *jwt.RegisteredClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour)) }, "k1", jwt.SigningMethodRS256, f.key),
		"위조 서명":      f.token(t, nil, "k1", jwt.SigningMethodRS256, other),
		"모르는 kid":    f.token(t, nil, "nope", jwt.SigningMethodRS256, f.key),
		"HS256 바꿔치기": f.token(t, nil, "k1", jwt.SigningMethodHS256, []byte("secret")),
		"형식 오류":      "not.a.jwt",
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := f.v.Verify(context.Background(), tok); err == nil {
				t.Fatal("거부해야 한다")
			}
		})
	}
}

func TestVerifyFailsClosedWhenCertsUnreachable(t *testing.T) {
	f := newFixture(t)
	tok := f.token(t, nil, "k1", jwt.SigningMethodRS256, f.key)
	f.v.certsURL = "http://127.0.0.1:1/unreachable"
	_, err := f.v.Verify(context.Background(), tok)
	if err == nil || !strings.Contains(err.Error(), "공개키") {
		t.Fatalf("공개키를 못 받으면 거부해야 한다, err=%v", err)
	}
}
