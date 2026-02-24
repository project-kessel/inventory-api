#!/bin/bash

set -euo pipefail

echo "Setting up pre-reqs..."
microdnf install -y jq

echo "Fetching required packages and RPM files..."
GRPCURL_VERSION=$(curl -sSfL https://api.github.com/repos/fullstorydev/grpcurl/releases/latest | jq -er '.tag_name')
ZED_VERSION=$(curl -sSfL https://api.github.com/repos/authzed/zed/releases/latest | jq -er '.tag_name')
# the tags fetched for grpcurl and zed will be in the format of 'vX.Y.Z', so bash substrings are used below to strip the 'v'
GRPCURL_VERSION=${GRPCURL_VERSION#v}
ZED_VERSION=${ZED_VERSION#v}

# Download grpcurl and zed RPM and checksums
curl -fLo /tmp/grpcurl_${GRPCURL_VERSION}_linux_amd64.rpm https://github.com/fullstorydev/grpcurl/releases/download/v${GRPCURL_VERSION}/grpcurl_${GRPCURL_VERSION}_linux_amd64.rpm
curl -fLo /tmp/grpcurl_checksums.txt https://github.com/fullstorydev/grpcurl/releases/download/v${GRPCURL_VERSION}/grpcurl_${GRPCURL_VERSION}_checksums.txt
curl -fLo /tmp/zed_${ZED_VERSION}_linux_amd64.rpm https://github.com/authzed/zed/releases/download/v${ZED_VERSION}/zed_${ZED_VERSION}_linux_amd64.rpm
curl -fLo /tmp/zed_checksums.txt https://github.com/authzed/zed/releases/download/v${ZED_VERSION}/checksums.txt

# Verify grpcurl and zed checksums
pushd /tmp
grep "grpcurl_${GRPCURL_VERSION}_linux_amd64.rpm" grpcurl_checksums.txt | sha256sum -c -
grep "zed_${ZED_VERSION}_linux_amd64.rpm" zed_checksums.txt | sha256sum -c -
popd

# Download latest 3.9.x kafka tools
# note currently hardcoded to only support kafka 3.9.x
# this should remain until clusters move to 4.x
BASE_URL="https://dlcdn.apache.org/kafka/"
KAFKA_VERSION=$(
  curl -fsSL "$BASE_URL" \
    | grep -Eo 'href="3\.9\.[0-9]+/' \
    | sed -E 's|href="||; s|/||' \
    | sort -V \
    | tail -n 1
)

curl -fLo /tmp/kafka.tgz https://dlcdn.apache.org/kafka/${KAFKA_VERSION}/kafka_${SCALA_VERSION}-${KAFKA_VERSION}.tgz

echo "Setting up EPEL Repository..."
rpm -ivh https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm

echo "Installing packages..."
microdnf install -y tar gzip wget bind-utils nmap-ncat openssl vim java-21-openjdk kcat util-linux less
rpm -ivh /tmp/grpcurl_${GRPCURL_VERSION}_linux_amd64.rpm /tmp/zed_${ZED_VERSION}_linux_amd64.rpm

mkdir -pv /opt/kafka
tar -xvf /tmp/kafka.tgz -C /opt/kafka --strip-components=1

# setup zed wrapper by renaming base zed cli
mv /usr/bin/zed /usr/bin/zed.original
mv /usr/local/bin/zed-wrapper.sh /usr/local/bin/zed

echo "Clean up..."
microdnf clean all -y
rm /tmp/*.rpm
rm /tmp/*.txt
