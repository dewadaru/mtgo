###############################################################################
# 20250326
# BUILD STAGE 

FROM golang:1.25.3-alpine3.22 AS builder

# Install build dependencies in one layer
RUN apk --no-cache add bash ca-certificates curl git make

WORKDIR /app

# Copy and download dependencies (cached layer)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build optimized static binary
# -trimpath: removes file system paths for reproducible builds
# -ldflags: reduces binary size and improves security
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath \
    -ldflags='-s -w -extldflags "-static"' \
    -o /mtg . || make -j$(nproc) static

###############################################################################
# RUNTIME STAGE

FROM scratch

# Copy SSL certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /mtg /mtg

# Copy config
COPY --from=builder /app/example.config.toml /config.toml

ENTRYPOINT ["/mtg"]
CMD ["run", "/config.toml"]