# Stage 1 — build a fully static binary (CGO required for tree-sitter).
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

RUN GOBIN=/usr/local/bin go install github.com/grpc-ecosystem/grpc-health-probe@latest

COPY . .
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w -linkmode external -extldflags '-static'" \
    -o codewalker ./cmd/codewalker

# Stage 2 — minimal runtime (static binary runs on distroless/static).
FROM gcr.io/distroless/static

COPY --from=builder /build/codewalker /codewalker
COPY --from=builder /usr/local/bin/grpc-health-probe /usr/local/bin/grpc-health-probe

EXPOSE 50051

ENTRYPOINT ["/codewalker"]
