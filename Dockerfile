# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS frontend
WORKDIR /ui
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-fund --no-audit
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /internal/httpapi/web ./internal/httpapi/web
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG REVISION=unknown
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-X main.version=${VERSION} -X main.revision=${REVISION}" -o /observatory ./cmd/observatory

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG REVISION=unknown
ARG SOURCE_URL
LABEL org.opencontainers.image.title="Observatory" \
      org.opencontainers.image.description="Local Docker monitoring and test investigation" \
      org.opencontainers.image.version=$VERSION \
      org.opencontainers.image.revision=$REVISION \
      org.opencontainers.image.source=$SOURCE_URL \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /observatory /observatory
COPY LICENSE /LICENSE
COPY THIRD_PARTY_NOTICES.md /THIRD_PARTY_NOTICES.md
COPY LICENSES /LICENSES
# Socket access requires host permissions. A read-only mount is not API authorization.
USER 0:0
ENTRYPOINT ["/observatory"]
