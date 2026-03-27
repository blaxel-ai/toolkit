FROM alpine:latest

WORKDIR /app

COPY --from=ghcr.io/blaxel-ai/sandbox:latest /sandbox-api /usr/local/bin/sandbox-api


ENTRYPOINT ["/usr/local/bin/sandbox-api"]