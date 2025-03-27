###############################################################################
# 20250326
# BUILD STAGE 

FROM golang:1.24.1-alpine3.21 AS builder

# Install build dependencies
RUN apk --no-cache add \
    bash \
    ca-certificates \
    curl \
    git \
    make

# Set working directory
WORKDIR /app

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the static binary
RUN make -j$(nproc) static

###############################################################################
# PACKAGE STAGE

FROM alpine:3.21

# Install netcat for healthcheck
RUN apk --no-cache add netcat-openbsd

# Copy SSL certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy the application binary
COPY --from=builder /app/mtg /mtg

# Copy the configuration file
COPY --from=builder /app/example.config.toml /config.toml

# Expose default port
EXPOSE 3128

# Health check
# nc is provided by netcat-openbsd package
HEALTHCHECK --interval=30s --timeout=3s \
    CMD nc -z localhost 3128

# Set entrypoint and default command
ENTRYPOINT ["/mtg"]
CMD ["run", "/config.toml"]
