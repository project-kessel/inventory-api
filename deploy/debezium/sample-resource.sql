INSERT INTO outbox_events (
    aggregatetype,
    aggregateid,
    type,
    payload
) VALUES (
    'kessel.resources',
    '1',
    'CreateNotificationsIntegration',
    '{"specversion":"1.0","id":"46e4e7d6-ff64-11ef-bd88-56aa371fad66","source":"http://localhost:8000","type":"redhat.inventory.resources.integration.created","subject":"/resources/integration/01958b52-01a0-7a1a-a50d-608ebf5a2a97","datacontenttype":"application/json","time":"2025-03-12T17:06:02.272666574Z","data":{"metadata":{"id":"01958b52-01a0-7a1a-a50d-608ebf5a2a97","resource_type":"integration","org_id":"","created_at":"2025-03-12T13:06:02.272666574-04:00","workspace_id":"1234"},"reporter_data":{"reporter_instance_id":"user@example.com","reporter_type":"NOTIFICATIONS","console_href":"","api_href":"","local_resource_id":"1234","reporter_version":""}}}'
);
