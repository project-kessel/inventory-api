#!/bin/bash

RESULT=`curl -k --data "grant_type=client_credentials&client_id=svc-test&client_secret=h91qw8bPiDj9R6VSORsI5TYbceGU5PMH" http://0.0.0.0:8084/realms/redhat-external/protocol/openid-connect/token`

if command -v jq > /dev/null 2>&1; then
  export ACCESS_TOKEN=$(echo "${RESULT}" | jq -r .access_token)
  printf "%s\n" ${ACCESS_TOKEN}
else
  echo $RESULT
fi