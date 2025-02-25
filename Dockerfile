FROM registry.access.redhat.com/ubi9/ubi-minimal:9.5-1739420147 AS builder

ARG TARGETARCH
USER root
RUN microdnf install -y tar gzip make which gcc gcc-c++ cyrus-sasl-lib findutils git go-toolset

WORKDIR /workspace

COPY go.mod go.sum ./

ENV CGO_ENABLED 1
RUN go mod download

COPY api ./api
COPY cmd ./cmd
COPY internal ./internal
COPY main.go Makefile ./
COPY data/resources ./data/resources

ARG VERSION
RUN VERSION=${VERSION} make build

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.5-1739420147

# installs RHEL fork of go to be able to validate with go tools for FIPS -- likely not needed long term
RUN microdnf install -y go-toolset

COPY --from=builder /workspace/bin/inventory-api /usr/local/bin/

EXPOSE 8081
EXPOSE 9081

USER 1001
ENV PATH="$PATH:/usr/local/bin"
ENV RESOURCE_DIR="data/resources"
ENTRYPOINT ["inventory-api"]

LABEL name="kessel-inventory-api" \
      version="0.0.1" \
      summary="Kessel inventory-api service" \
      description="The Kessel inventory-api service"
