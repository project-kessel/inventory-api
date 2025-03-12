CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE "public".outbox_events (
    id uuid DEFAULT uuid_generate_v4() constraint outbox_pk primary key,
    aggregatetype varchar(255) NOT NULL,
    aggregateid varchar(255) NOT NULL,
    type varchar(255) NOT NULL,
    payload jsonb NOT NULL
);
