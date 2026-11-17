# syntax=docker/dockerfile:1
#
# Compose builds this English fork from source. Release CI supplies precompiled
# binaries in dist/<TARGETARCH>/antifox and uses the default prebuilt stage.
ARG BINARY_SOURCE=prebuilt

FROM --platform=$BUILDPLATFORM node:22-bookworm-slim AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build:static

FROM --platform=$BUILDPLATFORM golang:1.26.3-bookworm AS source
ARG TARGETARCH
WORKDIR /src
COPY . .
COPY --from=frontend /src/web/out/ ./server/webui/dist/
RUN mkdir -p server/skillsdist \
    && cp -r skills/. server/skillsdist/ \
    && rm -rf server/skillsdist/report-generation \
    && CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
       go build -tags embedui -trimpath -o /dist/${TARGETARCH}/antifox ./cmd/antifox

FROM scratch AS prebuilt
COPY dist/ /dist/

FROM ${BINARY_SOURCE} AS binaries

FROM python:3.12-slim-bookworm
ARG TARGETARCH
# Common tools: ripgrep, curl, vim, and reconnaissance utilities; customize as needed.
# Install Node 20.x from NodeSource; bookworm ships Node 18, while Playwright needs >=20.
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates ripgrep curl wget vim git jq unzip \
      dnsutils iputils-ping netcat-openbsd whois nmap \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*
# Preinstall Playwright MCP/CLI and Chromium for browser MCP support in the container.
# The previous layer clears apt lists, but --with-deps needs them for Chromium dependencies.
# Run apt-get update first to prevent playwright install from failing with exit 100.
RUN apt-get update \
    && npm install -g @playwright/mcp@latest @playwright/cli@latest playwright@latest \
    && playwright-cli --help \
    && playwright install --with-deps chromium \
    && rm -rf /var/lib/apt/lists/*
# Reports require python-docx; Go invokes report_builder.py as a subprocess.
RUN pip install --no-cache-dir python-docx
WORKDIR /app
# Precompiled binary for the target architecture: dist/amd64/antifox or dist/arm64/antifox.
COPY --from=binaries /dist/${TARGETARCH}/antifox /app/antifox
RUN chmod +x /app/antifox
COPY skills/ /app/skills/
# Persistent data directory for local databases and jwt.key.
VOLUME ["/app/data"]
EXPOSE 8787 8788
ENTRYPOINT ["/app/antifox"]
CMD ["-addr", ":8787", "-proxy", ":8788"]
