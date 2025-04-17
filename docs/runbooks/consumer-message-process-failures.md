# Consumer Message Processing Failures

## Prerequisites

Remedations covered in this guide will require the following:
1) [zed cli](https://github.com/authzed/zed?tab=readme-ov-file#getting-started)
2) Access to running Inventory API pod logs
3) Access to a running container with Kafka CLI tools (covered in runbooks where needed)
4) Kafka connection information including bootstrap servers and authentication credentials (covered in runbooks where needed)

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
# port-forwad the spicedb service to your system
oc port-forward svc/kessel-relations-spicedb 50051:50051

# check the schema
zed schema read
```

The read command should dump out the entire SpiceDB schema. An error is provided if there are any connection issues or if no schema exists.


## Schema Related Issues

There are two known distinct reasons tuple creation could fail due to schema issues:
1) The schema definitions between Inventory API and Relations API are not in sync
2) The schema is valied but the tuple request values are malformed by Inventory API

### Inventory Consumer fails to Create/Modify a Relationship due to schema mismatch

**Example Error in Inventory API Logs**
```
msg=request failed: error creating tuple: rpc error: code = FailedPrecondition desc = error creating tuples: error writing relationships to SpiceDB: rpc error: code = FailedPrecondition desc = object definition `notifications/integration` not found
```

**Reason**
The error `object definition "OBJECT" not found` indicates the schema loaded by SpiceDB does not contain a defintion for the tuple request. This is likely due to a mismatch between Inventory API and Relations API schema definitions

**Verify**
You can verify if there is a schema mismatch one of two ways:

1) Check the loaded schema in Relations API for the object

```shell
zed schema read | grep <OBJECT>

# using the example error above
zed schema read | grep "notifications/integration"
```

If nothing is returned, the loaded schema is missing the expected definition

2) Compare the loaded schema to the expected schema

The schema is deployed via App Interface and is pulled from Github using a specifc commit hash (See [App Interface resource definition](https://gitlab.cee.redhat.com/service/app-interface/-/blob/master/data/services/insights/kessel/namespaces/kessel-prod.yml?ref_type=heads#L47)). When comparing the loaded schema to the expected schema, the commit ref listed in App Interface should be used

```shell
# capture the commit ref in app interface
COMMIT_REF=<value-in-app-interface>

# capture the loaded schema in SpiceDB
TMPDIR=$(mktemp -d) && pushd $TMPDIR

oc port-forward svc/kessel-relations-spicedb 50051:50051

zed schema read > loaded-schema.zed

# capture the schema def from the configmap
oc get cm spicedb-schema -o json | jq -r '.data."schema.zed"' > configmap.zed

# download the production schema from git using the commit ref
curl -O https://raw.githubusercontent.com/RedHatInsights/rbac-config/${COMMIT_REF}/configs/prod/schemas/schema.zed

# compare the downloaded schema to configmap schema
zed schema diff schema.zed configmap.zed

# compare the downloaded schema to the loaded schema from SpiceDB
zed schema diff schema.zed loaded-schema.zed

# when the issue is wrapped up, you can clean up with
popd && rm -rf $TMPDIR
```

The output of the `zed schema diff` commands will have no output when they match. Any differences will be shown which indicates a schema mismatch

If the downloaded schema and configmap schema dont match: there is an issue with the deployed configmap and the configmap must be redeployed with the correct hash via App Interface

If the download schema and configmap match but the loaded schema doesnt: SpiceDB has not properly loaded the configmap, restarting the Relations API pods should reload the configmap

If all schema definitions match and the expected object from the error log does not exist in any of them -- the issue is likely due to a malformed request. See [below](#inventory-consumer-fails-to-createmodify-a-relationship-due-to-schema-mismatch)

### Inventory Consumer fails due to malformed Tuple request

TBD
