FROM registry.access.redhat.com/ubi8/ubi-minimal:8.10 AS builder

ARG TARGETARCH
USER root
RUN microdnf install -y tar gzip make which gcc gcc-c++ cyrus-sasl-lib

# install platform specific go version
RUN curl -O -J  https://dl.google.com/go/go1.22.5.linux-${TARGETARCH}.tar.gz
RUN tar -C /usr/local -xzf go1.22.5.linux-${TARGETARCH}.tar.gz
RUN ln -s /usr/local/go/bin/go /usr/local/bin/go

WORKDIR /workspace

COPY . ./

ENV CGO_ENABLED 1
RUN go mod vendor
RUN make build

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.10

COPY --from=builder /workspace/bin/inventory-api /usr/local/bin/

EXPOSE 8080
EXPOSE 9080

USER 1001
ENV PATH="$PATH:/usr/local/bin"
ENTRYPOINT ["inventory-api"]

LABEL name="kessel-inventory-api" \
      version="0.0.1" \
      summary="Kessel inventory-api service" \
      description="The Kessel inventory-api service"
