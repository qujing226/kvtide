FROM golang:1.26 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount-type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" \
    -o /out/kvtide-server ./cmd/server
FROM alpine:latest

WORKDIR /app
COPY --from=build /out/kvtide-server /app/kvtide-server
EXPOSE 8800 8801
ENTRYPOINT ["/app/kvtide-server"]
