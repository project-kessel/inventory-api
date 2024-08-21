# Biz

The Use Cases in `biz` are basically pass-through calls to the `data` layer.

The models in these packages are internal to `inventory-api`.  If the system uses an ORM like `gorm`, they are
the structs that are annotated to reflect database table mappings.

A user request initially creates objects from the public `api` layer.  The `service` layer transforms them
into models from `biz`.  The reverse happens for responses:  the `service` layer translates `biz` models to
public `api` objects.

The `eventing` subsystem also has a relatively simple model in its `api` package, and the `data` subsystem
translates `biz` models to it before sending events.
