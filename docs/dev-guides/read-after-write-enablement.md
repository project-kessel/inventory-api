# Configuring Read-After-Write 

This guide walks through the setup of configuring read-after-write (r-a-w) enablement and sample configurations.

## Local Setup 
Modify [.inventory-api.yaml](../.inventory-api.yaml).
   - Edit the `consistency` section with desired changes:
     ```shell
     consumer:
       ...
     consistency:
       read-after-write-enabled: true # false == off for all service providers
       read-after-write-allowlist: [] # specify ["*"] to allow any request to optionally r-a-w
     ```
   - Ensure every request from a service provider can be optionally r-a-w enabled by updating the allowlist with their `reporter_id` field. Individual requests will still be required to toggle `write_visibility="IMMEDIATE"` to behave as r-a-w. For instance, for the Notifications service:
     ```shell
     read-after-write-enabled: true
     read-after-write-allowlist: ["NOTIFICATIONS"]
     ```
   - Rebuild inventory after making these changes.

## Ephemeral Setup
Modify [kessel-inventory-ephem.yaml](../deploy/kessel-inventory-ephem.yaml).
   - Follow the same steps as for [Local Setup](#local-setup), ensuring to update the `reporter_id` in the allowlist if necessary.
   - After changes are made, cycle the `inventory-api` pods or deploy them. For deployment instructions, refer to [ephemeral-testing.md](./ephemeral-testing.md).

## Stage/Prod Setup
 Changes should be managed via App Interface updates in a similar manner as [local](#local-setup) and [Ephemeral](#ephemeral-setup) setups but for the `inventory-api.yaml` data section.
   - After merging desired changes, pods will cycle to apply these modifications.


## Example Configurations
**Read After Write Enabled & All Service Providers**

inventory-api-config.yaml:
```shell
consumer:
    ...
consistency:
    read-after-write-enabled: true 
    read-after-write-allowlist: ["*"] # ALL requests can optionally be r-a-w
```

**Read After Write Enabled & Some Service Providers**

NOTE: Requests that explicitly request for r-a-w via the `write_visibility="IMMEDIATE"` toggle will be allowed ONLY if the SP is in the allow list.

inventory-api-config.yaml:
```shell
consumer:
    ...
consistency:
    read-after-write-enabled: true 
    read-after-write-allowlist: ["NOTIFICATIONS"] # Notifications requests can optionally be r-a-w
```

**Read After Write Enabled Globally (with Explicit Request)**

NOTE: With no SPs in the allowlist all requests will be fire-and-forget.

inventory-api-config.yaml:
```shell
consumer:
    ...
consistency:
    read-after-write-enabled: true 
    read-after-write-allowlist: []
```

**Read After Write Disabled Globally**

NOTE: All requests will behave as fire-and-forget.

inventory-api-config.yaml:
```shell
consumer:
    ...
consistency:
    read-after-write-enabled: false 
    read-after-write-allowlist: []
```