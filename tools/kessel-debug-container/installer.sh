#!/bin/bash

set -euo pipefail

echo "Installing DNF available packages..."
dnf install -y jq curl tar gzip vim-minimal bind-utils java-21-openjdk util-linux less

echo "Installing remaining packages via RPMs from mirrors..."
ZED_VERSION=$(curl -sSfL https://api.github.com/repos/authzed/zed/releases/latest | jq -er '.tag_name')
# the tag fetched for zed will be in the format of 'vX.Y.Z', so bash substring is used below to strip the 'v'
ZED_VERSION=${ZED_VERSION#v}

RPM_URLS=(
  "https://github.com/authzed/zed/releases/download/v${ZED_VERSION}/zed_${ZED_VERSION}_linux_amd64.rpm"
  "https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/librdkafka-${LIBRDKAFKA_VERSION}.el9.x86_64.rpm"
  "https://dl.fedoraproject.org/pub/epel/9/Everything/x86_64/Packages/k/kcat-${KCAT_VERSION}.el9.x86_64.rpm"
)

for URL in ${RPM_URLS[*]}; do
  dnf install -y $URL
done

echo "Installing Kafka tools..."
curl -fLo /tmp/kafka.tgz "https://dlcdn.apache.org/kafka/${KAFKA_VERSION}/kafka_${SCALA_VERSION}-${KAFKA_VERSION}.tgz"
mkdir -pv /opt/kafka
tar -xvf /tmp/kafka.tgz -C /opt/kafka --strip-components=1

# setup zed wrapper by renaming base zed cli
mv /usr/bin/zed /usr/bin/zed.original
mv /usr/local/bin/zed-wrapper.sh /usr/local/bin/zed

echo "Clean up..."
dnf clean all -y
