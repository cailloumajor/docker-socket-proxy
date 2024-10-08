# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.5.0 AS xx

FROM --platform=$BUILDPLATFORM golang:1.23.2-bookworm AS builder

COPY --from=xx / /

WORKDIR /usr/src/app

COPY go.mod go.sum *.go ./
COPY cmd ./cmd
COPY internal ./internal

ARG TARGETPLATFORM
ENV CGO_ENABLED=0
RUN xx-go build -o bin/ -v ./... && \
    xx-verify bin/*

# hadolint ignore=DL3006
FROM gcr.io/distroless/static-debian12

COPY --from=builder /usr/src/app/bin/* /usr/local/bin/

HEALTHCHECK CMD ["/usr/local/bin/healthcheck"]

EXPOSE 2375

USER nonroot
CMD ["/usr/local/bin/docker-socket-proxy", "-api-listen", "0.0.0.0:2375"]
