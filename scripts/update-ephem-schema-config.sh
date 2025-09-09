#!/bin/bash

if [ ! -f resources.tar.gz ]; then
  echo "Error: resources.tar.gz not found"
  exit 1
fi

REPO_ROOT=$(git rev-parse --show-toplevel)
export TAR_BASE64=$(cat $REPO_ROOT/resources.tar.gz | base64 -w 0)
if [ $? -ne 0 ]; then
  echo "Error: Failed to base64 encode the resoures tar file"
  exit 1
fi

yq -i '(.objects[] | select(.metadata.name == "resources-tarball") | .binaryData."resources.tar.gz") = strenv(TAR_BASE64)' $REPO_ROOT/deploy/kessel-inventory-ephem.yaml
if [ $? -ne 0 ]; then
  echo "Error: Failed to update deployment file"
  exit 1
fi
