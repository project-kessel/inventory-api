# Common Inventory
This repository implements a common inventory system with eventing.

```bash
make init
make api
make build
./bin/inventory-api migrate
./bin/inventory-api serve
```

```bash
curl -H "Authorization: Bearer 123" -H "Content-Type: application/json" --data "@data/host.json" http://localhost:8080/v1beta1/hosts
```
