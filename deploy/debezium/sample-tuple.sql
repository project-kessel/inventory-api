INSERT INTO outbox_events (
    aggregatetype,
    aggregateid,
    type,
    payload
) VALUES (
    'kessel.tuples',
    '1',
    'CreateTuple',
    '{"subject":{"subject":{"id":"my_workspace","type":{"name":"workspace","namespace":"rbac"}}},"relation":"t_workspace","resource":{"id":"my_integration","type":{"name":"integration","namespace":"notifications"}}}'
);
