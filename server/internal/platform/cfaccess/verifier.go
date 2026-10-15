// Package cfaccess 는 Cloudflare Access 가 붙여 주는 JWT 를 검증한다.
//
// Access 가 외부 접근의 정문이지만 ingress-nginx 는 Tunnel 말고도 닿는 경로가 있다(클러스터 내부,
// tailnet, 직접 접속 검증용 인증서). 그래서 서버가 모든 요청의 Access 토큰을 다시 확인한다.
// 공개키를 못 받아 오면 전부 거부한다(fail-closed).
package cfaccess

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	keysTTL       = time.Hour
	minRefreshGap = 30 * time.Second // 모르는 kid 가 계속 들어와도 공개키 조회를 이 간격 이상으로 안 늘린다
	fetchTimeout  = 5 * time.Second
	HeaderName    = "Cf-Access-Jwt-Assertion"
	CookieName    = "CF_Authorization"
)

var ErrNoToken = errors.New("access token 없음")

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type Verifier struct {
	issuer   string
	audience string
	certsURL string
	http     *http.Client

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// New 는 teamDomain(예: planmate.cloudflareaccess.com)과 Access 애플리케이션 AUD 로 검증기를 만든다.
func New(teamDomain, audience string) *Verifier {
	issuer := "https://" + teamDomain
	return &Verifier{
		issuer:   issuer,
		audience: audience,
		certsURL: issuer + "/cdn-cgi/access/certs",
		http:     &http.Client{Timeout: fetchTimeout},
	}
}

func (v *Verifier) Verify(ctx context.Context, token string) (*Claims, error) {
	if token == "" {
		return nil, ErrNoToken
	}
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			return v.key(ctx, kid)
		},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if k, ok := v.keys[kid]; ok && time.Since(v.fetchedAt) < keysTTL {
		return k, nil
	}
	// 키 회전 직후의 새 kid 이거나 캐시가 오래됐다. 너무 자주 조회하지 않게 간격을 둔다.
	if v.keys == nil || time.Since(v.fetchedAt) >= minRefreshGap {
		keys, err := v.fetch(ctx)
		if err != nil {
			if k, ok := v.keys[kid]; ok {
				return k, nil // 조회 실패 시 아직 가진 키는 계속 쓴다
			}
			return nil, fmt.Errorf("access 공개키 조회 실패: %w", err)
		}
		v.keys, v.fetchedAt = keys, time.Now()
	}
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("모르는 kid %q", kid)
}

func (v *Verifier) fetch(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.certsURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, err
	}
	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return nil, errors.New("공개키가 비어 있다")
	}
	return keys, nil
}
