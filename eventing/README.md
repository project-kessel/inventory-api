# Eventing

Each CRUD operation generates cloudevents from the `data` layer.  Events are sent using an `EventManager` and
a `Producer`.

To send an event, the `data` layer first uses the requester's identity and information about the resource to
look up a `Producer` from the `EventManager`.  The `Lookup` handles any complicated logic about selecting the
correct Kafka topic and captures the decision in the `Producer`.  The `data` layer then uses the `Producer` to
send the event.

This package contains two implementations.

1. `stdout` dumps `json` encoded events to `stdout`
2. `kafka` sends a cloudevent to a Kafka topic
