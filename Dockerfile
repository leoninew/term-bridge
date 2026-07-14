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
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-X gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/version.Version=${VERSION} -X gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/version.Commit=${COMMIT} -X gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/version.BuildTime=${BUILD_TIME}" \
    -o /out/termbridge ./cmd/termbridge

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /opt/termbridge
COPY --from=go-build /out/termbridge /usr/local/bin/termbridge
COPY --from=web-build /src/web/dist ./web/dist
COPY configs/ ./configs/
EXPOSE 80
VOLUME ["/var/lib/termbridge"]
ENTRYPOINT ["termbridge"]
CMD ["cloud"]
