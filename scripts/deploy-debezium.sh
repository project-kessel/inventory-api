#!/bin/bash

DB_SECRET=$(oc get secret kessel-inventory-db -o json)
DB_NAME=$(echo $DB_SECRET | jq -r '.data."db.name" | @base64d')
DB_HOSTNAME=$(echo $DB_SECRET | jq -r '.data."db.host" | @base64d')
DB_PORT="5432"
LOCAL_DB_PORT="5432"
DB_USER=$(echo $DB_SECRET | jq -r '.data."db.user" | @base64d')
DB_PASSWORD=$(echo $DB_SECRET | jq -r '.data."db.password" | @base64d')
KAFKA_CONNECT_INSTANCE=$(oc get kc -o jsonpath='{.items[*].metadata.name}')
BOOTSTRAP_SERVER=$(oc get svc -o json | jq -r '.items[] | select(.metadata.name | test("^env-ephemeral.*-kafka-bootstrap")) | "\(.metadata.name).\(.metadata.namespace).svc"')

oc process --local -f deploy/debezium/debezium-connector.yaml \
  -p DB_NAME=${DB_NAME} \
  -p DB_HOSTNAME=${DB_HOSTNAME} \
  -p DB_PORT=${DB_PORT} \
  -p DB_USER=${DB_USER} \
  -p DB_PASSWORD=${DB_PASSWORD} \
  -p KAFKA_CONNECT_INSTANCE=${KAFKA_CONNECT_INSTANCE} | oc apply -f -