// Package webui 는 React 빌드 결과물(web/dist)을 바이너리에 넣어 서빙한다.
//
// 빌드 시 web/dist 를 이 패키지의 dist/ 로 복사한다(Dockerfile, scripts/build.sh).
// 로컬 개발에서는 dist 가 비어 있고, 화면은 Vite dev server(:5173)가 맡는다.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

func Handler() http.Handler {
	dist, _ := fs.Sub(embedded, "dist")
	return newHandler(dist)
}

func newHandler(dist fs.FS) http.Handler {
	files := http.FileServerFS(dist)
	index, indexErr := fs.ReadFile(dist, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if indexErr != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("프론트엔드가 이 바이너리에 포함돼 있지 않다. 로컬 개발은 Vite dev server(http://localhost:5173)를 쓴다.\n"))
			return
		}

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if st, err := fs.Stat(dist, name); err == nil && !st.IsDir() {
				// Vite 가 내용 해시를 파일명에 넣는 assets/ 는 영구 캐시해도 된다.
				if strings.HasPrefix(name, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		// SPA: 나머지 경로는 모두 index.html. 새 배포가 바로 보이도록 캐시하지 않는다.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}
