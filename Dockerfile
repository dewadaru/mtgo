###############################################################################
# 20250326
# BUILD STAGE 

FROM golang:1.24.3-alpine3.21 AS builder

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
# FINAL STAGE

FROM alpine:3.21

LABEL maintainer="DD"

# Install runtime dependencies (ca-certificates is often needed for HTTPS)
RUN apk --no-cache add \
    ca-certificates \
    tzdata

# Create a non-root user and group
RUN addgroup -S mtg && adduser -S -G mtg -H -h /app mtg

# Set working directory
WORKDIR /app

# Copy SSL certificates from builder (if not already included by ca-certificates)
# COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy the application binary from the builder stage to a standard location
COPY --from=builder /app/mtg /usr/local/bin/mtg

# Copy the configuration file from the builder stage to a standard location
COPY --from=builder /app/example.config.toml /etc/mtg/config.toml

# Ensure the non-root user owns the application files and workdir if necessary
# RUN chown -R mtg:mtg /app /etc/mtg

# Switch to the non-root user
USER mtg

# Expose the default port (update if your application uses a different port)
# EXPOSE 2398

# Set entrypoint and default command
ENTRYPOINT ["/usr/local/bin/mtg"]
CMD ["run", "/etc/mtg/config.toml"]
