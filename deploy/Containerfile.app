FROM docker.io/library/alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY medvault-app /app/medvault-app
RUN chmod 0755 /app/medvault-app

EXPOSE 8095
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8095/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/medvault-app"]