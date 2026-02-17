#!/bin/bash

set -e

echo "Setting up pre-reqs..."
microdnf install -y jq

echo "Fetching required packages and RPM files..."
GRPCURL_VERSION=$(curl https://api.github.com/repos/fullstorydev/grpcurl/releases/latest  | jq -r '.tag_name')
ZED_VERSION=$(curl https://api.github.com/repos/authzed/zed/releases/latest | jq -r '.tag_name')
# the tags fetched for grpcurl and zed will be in the format of 'vX.Y.Z', so bash substrings are used below to strip the 'v'
curl -Lo /tmp/grpcurl.rpm https://github.com/fullstorydev/grpcurl/releases/download/v${GRPCURL_VERSION:1}/grpcurl_${GRPCURL_VERSION:1}_linux_amd64.rpm && \
curl -Lo /tmp/zed.rpm https://github.com/authzed/zed/releases/download/v${ZED_VERSION:1}/zed_${ZED_VERSION:1}_linux_amd64.rpm
curl -Lo /tmp/kafka.tgz https://dlcdn.apache.org/kafka/${KAFKA_VERSION}/kafka_${SCALA_VERSION}-${KAFKA_VERSION}.tgz


echo "Setting up EPEL Repository..."
rpm -ivh https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm

echo "Setting up Postgresql 17 Repository..."
rpm -ivh https://download.postgresql.org/pub/repos/yum/reporpms/EL-9-x86_64/pgdg-redhat-repo-latest.noarch.rpm

echo "Installing packages..."
microdnf install -y tar gzip wget bind-utils nmap-ncat openssl vim java-21-openjdk kcat postgresql17 util-linux less
rpm -ivh /tmp/grpcurl.rpm /tmp/zed.rpm

mkdir -pv /opt/kafka
tar -xvf /tmp/kafka.tgz -C /opt/kafka --strip-components=1

# setup zed wrapper by renaming base zed cli
mv /usr/bin/zed /usr/bin/zed.original
mv /usr/local/bin/zed-wrapper.sh /usr/local/bin/zed

echo "Clean up..."
microdnf clean all -y
rm /tmp/*.rpm
