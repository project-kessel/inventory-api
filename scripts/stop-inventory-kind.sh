#!/bin/bash
set -oe errexit

if ! command -v kind &> /dev/null
then
    echo "kind could not be found"
    exit
fi

echo "Deleting kind inventory-cluster"
kind delete clusters inventory-cluster
