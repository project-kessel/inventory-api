INSERT INTO outbox_events (
    aggregatetype,
    aggregateid,
    operation,
    payload
) VALUES (
    'kessel.tuples',
    '1',
    'created',
    '{"subject":{"subject":{"id":"1234","type":{"name":"workspace","namespace":"rbac"}}},"relation":"t_workspace","resource":{"id":"4321","type":{"name":"integration","namespace":"notifications"}}}'
);
