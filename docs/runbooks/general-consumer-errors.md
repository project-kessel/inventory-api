# General Consumer Errors

### Offset Out of Range

**Example Error in Logs**

```bash
%4|1761592217.689|OFFSET|inventory-consumer#consumer-1| [thrd:main]: outbox.event.kessel.tuples [0]: offset reset (at offset 8 (leader epoch 8), broker 1) to offset BEGINNING (leader epoch -1): fetch failed due to requested offset not available on the broker: Broker: Offset out of range
```

This error occurs when a consumer tries to read from an offset that no longer exists, often because the data has been deleted due to retention policies. To resolve it, you can reset the consumer's offset to the latest available message using the appropriate Kafka command.

To confirm the issue, check the consumer group's current offset and lag offset in Kafka (the [kessel-debug container](https://github.com/project-kessel/inventory-api/tree/main/tools/kessel-debug-container) will have all the tools needed to do so)

```bash
$ ./bin/kafka-consumer-groups.sh --bootstrap-server $BOOTSTRAP_SERVERS --command-config $KAFKA_AUTH_CONFIG --group inventory-consumer --describe

GROUP                TOPIC                       PARTITION  CURRENT-OFFSET  LOG-END-OFFSET  LAG    CONSUMER-ID
inventory-consumer   outbox.event.kessel.tuples  0          8               9               1      inventory-consumer-7991a648
```

Based on the above example and the error, the consumer is attempting to process offset 8 to catch up to the latest offset. If all events from the topic were removed, the underlying message with offset 8 won't exist anymore to process.

Kafka does not automatically reset offsets just because messages were deleted. The offset is simply a pointer to a position beyond what’s in the topic right now. To fix the issue, we need to reset the consumer group's current offset to match the latest.

# shift to latest offset
./bin/kafka-consumer-groups.sh --bootstrap-server $BOOTSTRAP_SERVERS --command-config $KAFKA_AUTH_CONFIG --group inventory-consumer --reset-offsets --to-latest --execute --topic outbox.event.kessel.tuples
```

Note, the `kafka-consumer-groups.sh` command generally expects the consumer to not be active in order to complete. It may be required to disable the consumer before shifting the offset. Consumer failures have a retry loop with backoff, executing the command in those waiting periods generally is sufficient. If the consumer needs to be disabled, it can be done through the Inventory API configmap (will require an App Interface PR to change it):

```yaml
consumer:
  enabled: false
```

When new messages arrive in the topic, Kafka appends new records starting from the next offset. Since the consumer group’s committed offset will be at the latest, it will resume from the last offset onward — meaning it will consume these new messages as soon as they appear.
