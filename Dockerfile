# syntax=docker/dockerfile:1.7

ARG NODE_IMAGE=node:22-bookworm-slim
ARG GO_IMAGE=golang:1.25-bookworm
ARG RUNTIME_IMAGE=debian:bookworm-slim
ARG LANNVENTORY_VERSION=0.1.0-beta.2

FROM ${NODE_IMAGE} AS frontend-build
WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM ${GO_IMAGE} AS backend-build
ARG LANNVENTORY_VERSION
WORKDIR /src

COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

COPY backend/ ./backend/
COPY --from=frontend-build /src/frontend/dist/assets/ ./backend/internal/web/public/assets/
COPY --from=frontend-build /src/frontend/dist/fs/public/ ./backend/internal/web/public/

RUN set -eux; \
    cd backend/internal/web/public/assets; \
    sed -i 's/assets/fs\/public\/assets/g' index.js; \
    sed -i 's|url(/assets/|url(/fs/public/assets/|g' index.css

RUN cd backend && \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w -X github.com/godlev/LANnventory/internal/version.Version=${LANNVENTORY_VERSION}" \
      -o /out/lannventory \
      ./cmd/LANnventory

FROM ${RUNTIME_IMAGE} AS runtime

ARG LANNVENTORY_VERSION
LABEL org.opencontainers.image.title="LANnventory" \
      org.opencontainers.image.description="Self-contained LAN inventory and presence monitoring UI" \
      org.opencontainers.image.source="https://github.com/godlev/LANnventory" \
      org.opencontainers.image.version="${LANNVENTORY_VERSION}" \
      org.opencontainers.image.licenses="MIT"

ENV HOST=0.0.0.0 \
    PORT=8840

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      arp-scan \
      ca-certificates \
      curl \
      tzdata; \
    rm -rf /var/lib/apt/lists/*; \
    mkdir -p /data/WatchYourLAN

COPY --from=backend-build /out/lannventory /usr/local/bin/lannventory
RUN ln -s /usr/local/bin/lannventory /usr/local/bin/lanventory && \
    ln -s /usr/local/bin/lannventory /usr/local/bin/watchyourlan

WORKDIR /data/WatchYourLAN
VOLUME ["/data/WatchYourLAN"]
EXPOSE 8840

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD curl -fsS "http://127.0.0.1:${PORT}/api/health" >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/lannventory"]
CMD ["-d", "/data/WatchYourLAN"]
