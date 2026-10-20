# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/decision-gateway ./cmd/decision-gateway

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/decision-gateway /decision-gateway
COPY decisions /etc/decision-gateway/decisions
ENV DG_LISTEN_ADDR=:8080 \
    DG_DEFINITIONS_DIR=/etc/decision-gateway/decisions
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/decision-gateway"]
