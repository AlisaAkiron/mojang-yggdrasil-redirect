# --- build stage: generate the docs site, then cross-compile a static api
# binary that embeds it ---
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY site ./site
RUN go run ./cmd/sitegen -src site -out site/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/mojang-yggdrasil-redirect ./cmd/api

FROM alpine:3.24 AS runtime
RUN addgroup -S -g 65532 app && adduser -S -D -H -u 65532 -G app app
ENV PORT=8787
COPY --from=build /out/mojang-yggdrasil-redirect /mojang-yggdrasil-redirect
EXPOSE 8787
USER app
ENTRYPOINT ["/mojang-yggdrasil-redirect"]
