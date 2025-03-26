###############################################################################
# BUILD STAGE

FROM golang:1.24.1-alpine3.21 AS build

# Install dependencies
RUN apk --no-cache add \
    bash \
    ca-certificates \
    curl \
    git \
    make

# Set working directory and copy source code
WORKDIR /app
COPY . .

# Build the static binary
RUN make -j4 static

###############################################################################
# PACKAGE STAGE

FROM scratch

# Copy necessary files from the build stage
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /app/mtg /mtg
COPY --from=build /app/example.config.toml /config.toml

# Set entrypoint and default command
ENTRYPOINT ["/mtg"]
CMD ["run", "/config.toml"]

# Add healthcheck
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/mtg", "healthcheck"] || exit 1