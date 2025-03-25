###############################################################################
# BUILD STAGE

FROM golang:1.24.1-alpine3.21 AS build

RUN set -x \
  && apk --no-cache --update add \
    bash \
    ca-certificates \
    curl \
    git \
    make

COPY . /app
WORKDIR /app

RUN set -x \
  && make -j 4 static


###############################################################################
# PACKAGE STAGE

FROM scratch

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD nc -z localhost 3128 || exit 1

ENTRYPOINT ["/mtg"]
CMD ["run", "/config.toml"]

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /app/mtg /mtg
COPY --from=build /app/example.config.toml /config.toml
