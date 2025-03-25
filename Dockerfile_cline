###############################################################################
# BUILD STAGE
###############################################################################
FROM golang:1.24.1-alpine3.21 AS builder

# Install build dependencies
RUN apk --no-cache --update add \
    bash \
    ca-certificates \
    curl \
    git \
    make

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application with optimizations
RUN make -j $(nproc) static

###############################################################################
# FINAL STAGE
###############################################################################
FROM alpine:3.21 AS final

# Add metadata labels
# LABEL org.opencontainers.image.title="MTG Proxy"
# LABEL org.opencontainers.image.description="Highly-opinionated MTPROTO proxy for Telegram"
# LABEL org.opencontainers.image.url="https://github.com/9seconds/mtg"
# LABEL org.opencontainers.image.source="https://github.com/9seconds/mtg"
# LABEL org.opencontainers.image.licenses="MIT"

# Create non-root user
RUN adduser -D -H -s /sbin/nologin mtg

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy binary and config from builder
COPY --from=builder /app/mtg /app/mtg
COPY --from=builder /app/example.config.toml /app/config.toml

# Create volume for custom configuration
VOLUME ["/app/config"]

# Expose default port (can be overridden in config)
EXPOSE 3128

# Use non-root user
USER mtg

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD nc -z localhost 3128 || exit 1

# Set entrypoint and default command
ENTRYPOINT ["/app/mtg"]
CMD ["run", "/app/config.toml"]
