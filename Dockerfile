# planmate-monitor: React 빌드 → Go 바이너리에 포함 → distroless 이미지.
#
# 운영 노드(oracle-arm)가 arm64 라 멀티아치로 빌드한다. 빌드 단계는 러너 아키텍처(BUILDPLATFORM)에서
# 돌고 Go 가 대상 아키텍처로 크로스 컴파일하므로 QEMU 에뮬레이션이 필요 없다.

FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web /web/dist ./internal/webui/dist
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/monitor ./cmd/monitor

# static: CA 인증서 포함(Cloudflare 공개키 HTTPS 조회), 셸 없음. nonroot(65532)로 실행한다.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/monitor /monitor
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/monitor"]
