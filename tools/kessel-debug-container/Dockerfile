FROM registry.access.redhat.com/ubi9/ubi-minimal:9.7-1771346502

USER root

# Kafka/Scala versions are hard set to ensure compatibility with current Kafka infra
ENV SCALA_VERSION=2.13 \
    KAFKA_VERSION=3.9.1

COPY ./installer.sh /tmp
COPY ./scripts/* /usr/local/bin/

RUN /tmp/installer.sh

# starts in Kafka Home for shorter command paths
WORKDIR /opt/kafka

CMD ["sleep", "7200"]
