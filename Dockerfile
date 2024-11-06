FROM registry.access.redhat.com/ubi8/ubi-minimal:8.10-1130 AS builder

ARG TARGETARCH
USER root
RUN microdnf install -y tar gzip make which gcc gcc-c++ cyrus-sasl-lib findutils git

# install platform specific go version
RUN curl -O -J  https://dl.google.com/go/go1.22.5.linux-${TARGETARCH}.tar.gz
RUN tar -C /usr/local -xzf go1.22.5.linux-${TARGETARCH}.tar.gz
RUN ln -s /usr/local/go/bin/go /usr/local/bin/go

WORKDIR /workspace

COPY go.mod go.sum ./

ENV CGO_ENABLED 1
RUN go mod download

COPY api ./api
COPY cmd ./cmd
COPY internal ./internal
COPY main.go Makefile ./

ARG VERSION
RUN VERSION=${VERSION} make build

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.10-1130

COPY --from=builder /workspace/bin/inventory-api /usr/local/bin/

EXPOSE 8081
EXPOSE 9081

USER 1001
ENV PATH="$PATH:/usr/local/bin"
ENTRYPOINT ["inventory-api"]

LABEL name="kessel-inventory-api" \
      version="0.0.1" \
      summary="Kessel inventory-api service" \
      description="The Kessel inventory-api service"
