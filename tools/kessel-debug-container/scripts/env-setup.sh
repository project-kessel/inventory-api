#!/bin/bash

CDAPPCONFIG="/cdapp/cdappconfig.json"
export KAFKA_AUTH_CONFIG=/tmp/config.props

echo "Setting up Kafka Connection Info..."
# ensure we properly provide a comma-separeated list if there are more than one
export BOOTSTRAP_SERVERS=$(cat $CDAPPCONFIG | jq -r '[.kafka.brokers[] | "\(.hostname):\(.port)"] | join(",")')


# check if auth config info exists for setting up the auth config file as needed
# only the first broker is checked (if multiple) as we wont have multiple brokers from different clusters ever for a service
if [[ $(cat $CDAPPCONFIG | jq '.kafka.brokers[0] | has("sasl")') == "true" ]]; then
    export KAFKA_USERNAME=$(cat $CDAPPCONFIG | jq -r '.kafka.brokers[0].sasl.username')
    export KAFKA_PW=$(cat $CDAPPCONFIG | jq -r '.kafka.brokers[0].sasl.password')
fi

if [[ ! -z "$KAFKA_USERNAME" ]] && [[ ! -z "$KAFKA_PW" ]]; then
    echo "Setting up Kafka auth config..."
    cat <<EOF > $KAFKA_AUTH_CONFIG
sasl.mechanism=SCRAM-SHA-512
security.protocol=SASL_SSL
sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required username="$KAFKA_USERNAME" password="$KAFKA_PW";
EOF
fi

echo "Kafka Setup Complete!"
echo "Setting up Postgres Connection Info..."

export PGHOST=$(cat $CDAPPCONFIG | jq -r '.database.hostname')
export PGPORT=$(cat $CDAPPCONFIG | jq -r '.database.port')
export PGDATABASE=$(cat $CDAPPCONFIG | jq -r '.database.name')
export PGUSER=$(cat $CDAPPCONFIG | jq -r '.database.username')
export PGPASSWORD=$(cat $CDAPPCONFIG | jq -r '.database.password')

echo "Postgres Setup Complete!"
