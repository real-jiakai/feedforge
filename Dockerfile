# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/feedforge .

# ---- runtime stage ----
FROM alpine:3.22

# su-exec drops privileges in the entrypoint after fixing volume ownership.
RUN apk add --no-cache ca-certificates tzdata wget su-exec \
    && addgroup -S feedforge && adduser -S feedforge -G feedforge \
    && mkdir -p /data && chown feedforge:feedforge /data

COPY --from=build /out/feedforge /usr/local/bin/feedforge
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

ENV FEEDFORGE_ADDR=:8080 \
    FEEDFORGE_DATA=/data
EXPOSE 8080
VOLUME /data

# Probe whatever port FEEDFORGE_ADDR actually names, not a hardcoded one.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${FEEDFORGE_ADDR##*:}/healthz" || exit 1

# Starts as root only to chown a bind-mounted /data, then execs as the
# unprivileged feedforge user. See docker-entrypoint.sh.
ENTRYPOINT ["docker-entrypoint.sh"]
