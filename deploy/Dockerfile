# Stage 1 — build a fully static binary (CGO required for tree-sitter).
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w -linkmode external -extldflags '-static'" \
    -o codewalker ./cmd/codewalker

# Stage 2 — minimal runtime (static binary runs on distroless/static).
FROM gcr.io/distroless/static

COPY --from=builder /build/codewalker /codewalker

EXPOSE 50051

ENTRYPOINT ["/codewalker"]
