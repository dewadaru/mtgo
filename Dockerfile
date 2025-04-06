###############################################################################
# 20250326
# BUILD STAGE 

FROM golang:1.24.2-alpine3.21 AS builder

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

FROM scratch

# Copy SSL certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy the application binary
COPY --from=builder /app/mtg /mtg

# Copy the configuration file
COPY --from=builder /app/example.config.toml /config.toml

# Set entrypoint and default command
ENTRYPOINT ["/mtg"]
CMD ["run", "/config.toml"]
