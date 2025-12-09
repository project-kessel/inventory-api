-- Create "common_representations" table
CREATE TABLE "common_representations" (
  "data" jsonb NULL,
  "resource_id" uuid NOT NULL,
  "version" bigint NOT NULL,
  "reported_by_reporter_type" character varying(128) NULL,
  "reported_by_reporter_instance" character varying(128) NULL,
  "transaction_id" character varying(128) NULL,
  "created_at" timestamptz NULL,
  PRIMARY KEY ("resource_id", "version"),
  CONSTRAINT "chk_common_representations_version" CHECK (version >= 0)
);
-- Create index "ux_common_reps_txid_nn" to table: "common_representations"
CREATE UNIQUE INDEX "ux_common_reps_txid_nn" ON "common_representations" ("transaction_id") WHERE ((transaction_id IS NOT NULL) AND ((transaction_id)::text <> ''::text));
-- Create "outbox_events" table
CREATE TABLE "outbox_events" (
  "id" uuid NOT NULL,
  "aggregatetype" character varying(255) NOT NULL,
  "aggregateid" character varying(255) NOT NULL,
  "operation" character varying(255) NOT NULL,
  "txid" character varying(255) NULL,
  "payload" jsonb NULL,
  PRIMARY KEY ("id")
);
-- Create "reporter_resources" table
CREATE TABLE "reporter_resources" (
  "id" uuid NOT NULL,
  "local_resource_id" character varying(256) NOT NULL,
  "reporter_type" character varying(128) NOT NULL,
  "resource_type" character varying(128) NOT NULL,
  "reporter_instance_id" character varying(256) NOT NULL,
  "resource_id" uuid NOT NULL,
  "api_href" character varying(512) NOT NULL,
  "console_href" character varying(512) NULL,
  "representation_version" bigint NOT NULL,
  "generation" bigint NOT NULL,
  "tombstone" boolean NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "reporter_resource_key_idx" to table: "reporter_resources"
CREATE UNIQUE INDEX "reporter_resource_key_idx" ON "reporter_resources" ("local_resource_id", "reporter_type", "resource_type", "reporter_instance_id", "representation_version", "generation");
-- Create index "reporter_resource_resource_id_idx" to table: "reporter_resources"
CREATE INDEX "reporter_resource_resource_id_idx" ON "reporter_resources" ("resource_id");
-- Create index "reporter_resource_search_idx" to table: "reporter_resources"
CREATE INDEX "reporter_resource_search_idx" ON "reporter_resources" ("local_resource_id", "reporter_type", "resource_type", "reporter_instance_id");
-- Create "resource" table
CREATE TABLE "resource" (
  "id" uuid NOT NULL,
  "type" character varying(128) NOT NULL,
  "common_version" bigint NULL,
  "ktn" character varying(1024) NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_resource_common_version" CHECK (common_version >= 0)
);
-- Create "reporter_representations" table
CREATE TABLE "reporter_representations" (
  "data" jsonb NULL,
  "reporter_resource_id" uuid NOT NULL,
  "version" bigint NOT NULL,
  "generation" bigint NOT NULL,
  "reporter_version" character varying(128) NULL,
  "common_version" bigint NULL,
  "transaction_id" character varying(128) NULL,
  "tombstone" boolean NOT NULL,
  "created_at" timestamptz NULL,
  PRIMARY KEY ("reporter_resource_id", "version", "generation"),
  CONSTRAINT "fk_reporter_representations_reporter_resource" FOREIGN KEY ("reporter_resource_id") REFERENCES "reporter_resources" ("id") ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT "chk_reporter_representations_common_version" CHECK (common_version >= 0),
  CONSTRAINT "chk_reporter_representations_generation" CHECK (generation >= 0),
  CONSTRAINT "chk_reporter_representations_version" CHECK (version >= 0)
);
-- Create index "ux_reporter_reps_txid_nn" to table: "reporter_representations"
CREATE UNIQUE INDEX "ux_reporter_reps_txid_nn" ON "reporter_representations" ("transaction_id") WHERE ((transaction_id IS NOT NULL) AND ((transaction_id)::text <> ''::text));
