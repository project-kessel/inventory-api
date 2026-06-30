FROM registry.access.redhat.com/hi/core-runtime:2.42-openssl-fips-builder

USER root

# Kafka/Scala versions are hard set to ensure compatibility with current Kafka infra
# Kafka is hardcoded for 3.9.x in the installer script
ENV SCALA_VERSION=2.13

# librdkafka is required by kcat -- versions below represent the latest from mirrors
# see the README tools/kessel-debug-container/README.md for details on updating
ENV LIBRDKAFKA_VERSION=1.6.1-102
ENV KCAT_VERSION=1.7.1-1


COPY ./installer.sh /tmp
COPY ./scripts/* /usr/local/bin/

RUN /tmp/installer.sh

WORKDIR /opt/kafka

USER 1001

CMD ["sleep", "7200"]
