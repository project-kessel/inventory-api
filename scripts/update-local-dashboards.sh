#!/bin/bash

# check for yq
YQ=$(command -v yq)

# Use gawk with inplace extension if available, otherwise use temp file method
if command -v gawk >/dev/null 2>&1; then
    awk_inplace() {
        local file="$1"
        shift
        gawk -i inplace "$@" "$file"
    }
else
    awk_inplace() {
        local file="$1"
        shift
        awk "$@" "$file" > "${file}.tmp" && mv "${file}.tmp" "$file"
    }
fi


if [[ -z "$YQ" ]]; then
    echo "ERROR: yq cli required for this task"
    echo "Make sure to install it first"
    echo "See: https://github.com/mikefarah/yq?tab=readme-ov-file#install"
    exit 1
fi

for file in $(ls ./dashboards/); do
    DEST="./development/configs/monitoring/dashboards/${file%.*}.json"
    yq '.data' ./dashboards/$file | tail -n +2 > $DEST
    yq -iP '(.templating.list[] | select(.name == "datasource") | .regex) = "/(prometheus.*)/"' $DEST
    yq -iP '(.templating.list[] | select(.name == "kafka_datasource") | .regex) = "/(prometheus.*)/"' $DEST
    awk_inplace "$DEST" '{gsub(/namespace=\\"\$namespace\\",/, "")}1'
    awk_inplace "$DEST" '{gsub(/service=~/, "job=~")}1'

done

