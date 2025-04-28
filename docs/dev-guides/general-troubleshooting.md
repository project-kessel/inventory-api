# General Troubleshooting Guide for Local and Ephemeral Development


## Tuple Failures and Schema Related Issues

There are two known distinct reasons tuple creation could fail due to schema issues:
1) The schema definitions between Inventory API and Relations API are not in sync
2) The schema is valid but the tuple request values are malformed by Inventory API

For #1, the process to fix this issue is well covered in our [runbook](../runbooks/consumer-message-process-failures.md#inventory-consumer-fails-to-createmodify-a-relationship-due-to-schema-mismatch) with some minor differences:

1) The preshared token secret for setting up the Zed cli is stored in a different secret, which can be found in Relations API deploy files or compose file
2) The schema configmap is hardcoded into the epehmeral deployment manifest instead of loaded from upstream source

For #2, the process is also similar to our [runbook](../runbooks/consumer-message-process-failures.md#inventory-consumer-fails-due-to-malformed-tuple-request) but there are some extra options that may be simpler:

1) Instead of leveraing the ConsoleDot debug pod, you can rsh/exec into the Kafka Connect pod and run a similar command

```shell
/opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server <BOOTSTRAP_SERVER> --group inventory-consumer --reset-offsets --shift-by 1 --execute --topic outbox.event.kessel.tuples
```

2) If you'd rather just remove the messages from the queue, you can also use the same Connect pod above and run the delete records command:

> [!WARNING]
> Deleting records is dangerous and should never be done in stage or prod. This is a quick solution for local testing or ephemeral testing when you don't care about the data in Kafka

```shell
# Offset number is the desired offset to remove records up to. A good starting point is the offset number mentioned in error logs in inventory API. Setting the offset value to '-1' will remove all messages in the queue
/opt/kafka/bin/kafka-delete-records.sh --bootstrap-server <BOOTSTRAP-SERVER> --offset-json-file <(echo '{"partitions":[{"topic":"outbox.event.kessel.tuples","partition":0,"offset":<OFFSET-NUMBER>}],"version":1}')
