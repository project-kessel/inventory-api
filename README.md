# Common Inventory
This repository implements a common inventory system with eventing.

```bash
make init
make api
make build
./bin/inventory-api migrate --config .inventory-api.yaml
./bin/inventory-api serve --config .inventory-api.yaml
```

```bash
curl -H "Authorization: Bearer 123" -H "Content-Type: application/json" --data "@data/host.json" http://localhost:8080/api/inventory/v1beta1/hosts
```
