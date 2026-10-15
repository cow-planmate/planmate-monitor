package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cow-planmate/planmate-monitor/server/internal/platform/cfaccess"
)

// TokenVerifier 는 Cloudflare Access 토큰을 검증한다. 검증을 끈 환경(로컬)에서는 nil 이다.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (*cfaccess.Claims, error)
}

type ctxKey int

const emailKey ctxKey = iota

// 프로브 경로는 인증·요청 로그에서 뺀다. kubelet 은 Access 토큰이 없고, 몇 초마다 찍히는 로그는 소음이다.
func isProbe(p string) bool { return p == "/livez" || p == "/readyz" }

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic", "path", r.URL.Path, "panic", v)
				writeError(w, http.StatusInternalServerError, "서버 내부 오류")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isProbe(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		email, _ := r.Context().Value(emailKey).(string)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"ms", time.Since(start).Milliseconds(), "user", email)
	})
}

// accessAuth 는 Access 를 거쳐 온 요청인지 확인한다. 토큰은 헤더가 정식이고, 쿠키는 보조다.
func accessAuth(v TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if v == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isProbe(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			token := r.Header.Get(cfaccess.HeaderName)
			if token == "" {
				if c, err := r.Cookie(cfaccess.CookieName); err == nil {
					token = c.Value
				}
			}
			claims, err := v.Verify(r.Context(), token)
			if err != nil {
				slog.Warn("access denied", "path", r.URL.Path, "remote", r.RemoteAddr, "err", err)
				if strings.HasPrefix(r.URL.Path, "/api/") {
					writeError(w, http.StatusUnauthorized, "인증되지 않은 요청")
				} else {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
				}
				return
			}
			ctx := context.WithValue(r.Context(), emailKey, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
