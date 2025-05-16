#!/bin/bash

# check for yq
YQ=$(command -v yq)


if [[ -z "$YQ" ]]; then
    echo "ERROR: yq cli required for this task"
    echo "Make sure to install it first"
    echo "See: https://github.com/mikefarah/yq?tab=readme-ov-file#install"
    exit 1
fi

for file in $(ls ./dashboards/); do
    DEST="./development/configs/monitoring/dashboards/${file%.*}.json"
    yq '.data' ./dashboards/$file | tail -n +2 > $DEST
    yq -iP '(.templating.list[] | select(.name == "datasource") | .regex) = "/(prometheus)/"' $DEST
done

