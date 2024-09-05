#!/bin/bash

COMMIT=${1:-master}

echo 'schema: |-'
curl -s https://raw.githubusercontent.com/RedHatInsights/kessel-config/master/schema.zed | sed 's/^/  /'
echo
