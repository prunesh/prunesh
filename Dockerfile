FROM golang:1.26-alpine AS builder

WORKDIR /build

# Download deps first (layer cache)
COPY go.mod go.sum ./
RUN go mod download

# Build — CGO_ENABLED=0 produces a fully static binary (required for scratch)
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o prunesh ./cmd/prunesh/

# Final image — scratch: no shell, no OS, just the binary
FROM scratch

COPY --from=builder /build/prunesh /prunesh

ENTRYPOINT ["/prunesh"]
