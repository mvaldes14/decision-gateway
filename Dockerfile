# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS build
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /out/decision-gateway ./cmd/decision-gateway

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
LABEL org.opencontainers.image.title="Decision Gateway" \
      org.opencontainers.image.description="Provider-independent decision contract gateway" \
      org.opencontainers.image.source="https://github.com/mvaldes14/decision-gateway" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /out/decision-gateway /decision-gateway
COPY decisions /etc/decision-gateway/decisions
ENV DG_LISTEN_ADDR=:8080 \
    DG_DEFINITIONS_DIR=/etc/decision-gateway/decisions
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/decision-gateway"]
