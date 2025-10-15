#!/bin/bash

set -e

echo "Fetching required packages and RPM files..."
curl -Lo /tmp/grpcurl.rpm https://github.com/fullstorydev/grpcurl/releases/download/v${GRPCURL_VERSION}/grpcurl_${GRPCURL_VERSION}_linux_amd64.rpm && \
curl -Lo /tmp/zed.rpm https://github.com/authzed/zed/releases/download/v${ZED_VERSION}/zed_${ZED_VERSION}_linux_amd64.rpm
curl -Lo /tmp/kafka.tgz https://dlcdn.apache.org/kafka/${KAFKA_VERSION}/kafka_${SCALA_VERSION}-${KAFKA_VERSION}.tgz

echo "Setting up EPEL..."
rpm -ivh https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm

echo "Installing packages..."
microdnf install -y tar gzip wget bind-utils jq nmap-ncat openssl vim java-17-openjdk kcat
rpm -iv /tmp/grpcurl.rpm /tmp/zed.rpm

mkdir -pv /opt/kafka
tar -xvf /tmp/kafka.tgz -C /opt/kafka --strip-components=1
ln -s /opt/kafka/bin/* ./bin

# setup zed wrapper by renaming base zed cli
mv /usr/bin/zed /usr/bin/zed.original
mv /usr/local/bin/zed-wrapper.sh /usr/local/bin/zed

echo "Clean up..."
microdnf clean all -y
rm /tmp/*.rpm
