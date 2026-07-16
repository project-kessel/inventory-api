# Build stage — the Go toolchain embeds the validated FIPS module
# in all binaries automatically.
FROM registry.access.redhat.com/hi/go:1.26.5-fips AS builder

ARG TARGETARCH

WORKDIR /workspace

COPY go.mod go.sum ./

RUN go mod download

COPY api ./api
COPY cmd ./cmd
COPY internal ./internal
COPY main.go Makefile ./

ARG VERSION
RUN VERSION=${VERSION} make build

# Runtime stage — set GODEBUG so the binary runs in FIPS mode.
FROM registry.access.redhat.com/hi/core-runtime:2.42-openssl-fips
WORKDIR /
COPY --from=builder /workspace/bin/inventory-api /usr/local/bin/

EXPOSE 8081
EXPOSE 9081
ENV GODEBUG=fips140=on

USER 1001
ENV PATH="$PATH:/usr/local/bin"
ENTRYPOINT ["inventory-api"]

LABEL name="kessel-inventory-api" \
      version="0.0.1" \
      summary="Kessel inventory-api service" \
      description="The Kessel inventory-api service"
