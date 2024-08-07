# Common Inventory
This repository implements a common inventory system with eventing.

```bash
make init
make api
make build
make migrate
./bin/inventory-api serve --config .inventory-api.yaml
```


## Example Usage

To add hosts to the inventory, use the following `curl` command:

```bash
curl -H "Authorization: Bearer 1234" -H "Content-Type: application/json" --data "@data/host.json" http://localhost:8080/api/inventory/v1beta1/rhelHosts
```
