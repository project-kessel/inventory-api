# Manual Inventory DB replication to SpiceDB

## Prerequisites

Remediations covered in this guide will require the following:
1) [zed cli](https://github.com/authzed/zed?tab=readme-ov-file#getting-started)
2) Some python3 version installed.
3) Access to the portforwarding into the running cluster. 

### Configuring Zed CLI

To install the `zed` CLI, refer to [Authzed's README](https://github.com/authzed/zed?tab=readme-ov-file#getting-started) for specific instructions based on your OS

In order to access SpiceDB using the `zed` cli, you'll need to configure a context for the SpiceDB endpoint. This will require a preshared token available in a Kubernetes secret on cluster.

To configure the context:

```shell
# capture the preshared key from spicedb-config secret
PSTK=$(oc get secret spicedb-config -o jsonpath='{.data.preshared_key}' | base64 -d)

# create the context with zed cli -- change ENV to the ENV you are targeting (stage, prod)
zed context set <ENV>-spicedb localhost:50051 $PSTK --insecure

# verify the context was created and is the current context
zed context list
```

To confirm the context is working:

```shell
# port-forward the spicedb service to your system
oc port-forward svc/kessel-relations-spicedb 50051:50051

# check the schema
zed schema read
```

The read command should dump out the entire SpiceDB schema. An error is provided if there are any connection issues or if no schema exists.

## Running the migration script.

We will need a dump of the offset messages that need to be reprocessed.

Save these messages as some file like `kafkadump.txt`.

With our zed context setup to stage and portforwarding into the spicedb service, we can execute our [migration script](/docs/manual_migration_to_spicedb.py). Move the migration script to the same level as your `kafkadump.txt` file.

Run 
```shell
python3 manual_migration_to_spicedb.py kafkadump.txt
```