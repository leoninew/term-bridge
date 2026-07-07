FROM node:24-bookworm-slim AS web-build
WORKDIR /src/web
COPY web/package.json web/yarn.lock ./
RUN yarn install --frozen-lockfile
COPY web/ ./
RUN yarn build

FROM golang:1.25-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY migrations/ ./migrations/
RUN CGO_ENABLED=0 go build -o /out/termbridge ./cmd/termbridge

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /opt/termbridge
COPY --from=go-build /out/termbridge /usr/local/bin/termbridge
COPY --from=web-build /src/web/dist ./web/dist
COPY configs/ ./configs/
ENV TERMBRIDGE_CLOUD__LISTEN_URL=http://0.0.0.0:80 \
    TERMBRIDGE_CLOUD__STATIC_DIR=/opt/termbridge/web/dist \
    TERMBRIDGE_CLOUD__PUBLIC_URL=http://localhost \
    TERMBRIDGE_RUNTIME__STATE_DIR=/var/lib/termbridge
EXPOSE 80
VOLUME ["/var/lib/termbridge"]
ENTRYPOINT ["termbridge"]
CMD ["cloud"]
