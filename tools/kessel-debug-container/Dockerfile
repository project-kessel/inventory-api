FROM registry.access.redhat.com/ubi9/ubi-minimal:9.7-1769056855

USER root

ENV GRPCURL_VERSION=1.9.3 \
    ZED_VERSION=0.31.0 \
    SCALA_VERSION=2.13 \
    KAFKA_VERSION=3.9.0

COPY ./installer.sh /tmp
COPY ./scripts/* /usr/local/bin/

RUN /tmp/installer.sh

# starts in Kafka Home for shorter command paths
WORKDIR /opt/kafka

CMD ["sleep", "7200"]
