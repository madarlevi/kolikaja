# syntax=docker/dockerfile:1
# SPDX-License-Identifier: AGPL-3.0-only
# Copyright (C) 2026 madarlevi
#
# KoliKaja API image: static Go binaries (api, migrate, admin) on distroless "nonroot".
# Build context: the repository root (see docker-compose.yml).
# Base images are pinned by tag; to pin by digest, replace the tag with
# "<image>@sha256:<digest>" (docker buildx imagetools inspect <image:tag> prints it).
# The image carries LICENSE, THIRD_PARTY_LICENSES.md and the licence files of every Go
# module linked into the binaries under /usr/share/doc/kolikaja.

FROM golang:1.27.4-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-trimpath
COPY apps/api/go.mod apps/api/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY apps/api/ ./
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api && \
    go build -ldflags "-s -w" -o /out/migrate ./cmd/migrate && \
    go build -ldflags "-s -w" -o /out/admin ./cmd/admin
# Licence texts of the linked modules (MIT/BSD require them to accompany binaries).
RUN --mount=type=cache,target=/go/pkg/mod \
    mkdir -p /licenses/go && \
    cp "$(go env GOROOT)/LICENSE" "$(go env GOROOT)/PATENTS" /licenses/go/ && \
    go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}@{{.Version}} {{.Dir}}{{end}}{{end}}' ./cmd/... | sort -u | \
    while read -r mod dir; do \
      mkdir -p "/licenses/$mod"; \
      for f in "$dir"/LICENSE* "$dir"/LICENCE* "$dir"/COPYING* "$dir"/NOTICE* "$dir"/PATENTS*; do \
        if [ -f "$f" ]; then cp "$f" "/licenses/$mod/"; fi; \
      done; \
      if [ -z "$(ls -A "/licenses/$mod")" ]; then echo "no licence file for $mod" >&2; exit 1; fi; \
    done

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.title="KoliKaja API" \
      org.opencontainers.image.source="https://github.com/madarlevi/kolikaja" \
      org.opencontainers.image.licenses="AGPL-3.0-only"
COPY LICENSE THIRD_PARTY_LICENSES.md /usr/share/doc/kolikaja/
COPY --from=build /licenses/ /usr/share/doc/kolikaja/licenses/
COPY --from=build /out/ /app/
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=6 CMD ["/app/api", "healthcheck"]
CMD ["/app/api"]
