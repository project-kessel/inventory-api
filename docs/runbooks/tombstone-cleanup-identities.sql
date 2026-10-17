\set ON_ERROR_STOP on

-- Required psql variables (there are deliberately no retention defaults):
--   evaluation_time_utc   frozen UTC evaluation time shown by the job preview
--   cutoff_utc            strict cutoff shown by that same preview
--   reporter_type_filter  normalized filter from that preview, or empty for all types
--   bounded_row_limit     explicit batch-size * max-batches from that preview
SELECT CASE WHEN
    :'evaluation_time_utc'::timestamptz > :'cutoff_utc'::timestamptz
    AND :'bounded_row_limit'::bigint > 0
    AND (
        :'reporter_type_filter' = ''
        OR :'reporter_type_filter' = lower(btrim(:'reporter_type_filter'))
    )
    THEN 'true' ELSE 'false' END AS inputs_valid
\gset
\if :inputs_valid
\else
\echo 'Invalid identity-query inputs: pass the frozen UTC evaluation/cutoff, a normalized reporter filter, and a positive explicit row limit.'
\quit 3
\endif

BEGIN TRANSACTION ISOLATION LEVEL SERIALIZABLE READ ONLY;

WITH parameters AS MATERIALIZED (
    SELECT
        :'evaluation_time_utc'::timestamptz AS evaluation_time_utc,
        :'cutoff_utc'::timestamptz AS cutoff_utc,
        NULLIF(:'reporter_type_filter', '')::text AS reporter_type_filter,
        :'bounded_row_limit'::bigint AS bounded_row_limit
),
age_candidates AS (
    SELECT rr.id AS reporter_resource_id, rr.resource_id, rr.updated_at,
           rr.representation_version, rr.generation, rr.local_resource_id,
           rr.reporter_type, rr.resource_type, rr.reporter_instance_id
    FROM reporter_resources AS rr
    CROSS JOIN parameters AS p
    WHERE rr.tombstone IS TRUE
      AND rr.updated_at < p.cutoff_utc
      AND (p.reporter_type_filter IS NULL OR rr.reporter_type = p.reporter_type_filter)
),
classified_candidates AS (
    SELECT candidate.*,
           CASE
               WHEN root.id IS NULL THEN 'missing_root'
               WHEN root.type IS DISTINCT FROM candidate.resource_type THEN 'root_type_mismatch'
               WHEN NULLIF(BTRIM(candidate.local_resource_id), '') IS NULL OR
                    NULLIF(BTRIM(candidate.reporter_type), '') IS NULL OR
                    NULLIF(BTRIM(candidate.resource_type), '') IS NULL OR
                    NULLIF(BTRIM(candidate.reporter_instance_id), '') IS NULL THEN 'invalid_reporter_key'
               WHEN terminal.reporter_resource_id IS NULL THEN 'missing_terminal_representation'
               WHEN terminal.tombstone IS DISTINCT FROM TRUE OR
                    terminal.common_version IS NOT NULL OR
                    terminal.reporter_version IS NOT NULL OR
                    (terminal.data IS NOT NULL AND terminal.data <> 'null'::jsonb) THEN 'inconsistent_terminal_representation'
               WHEN EXISTS (
                   SELECT 1
                   FROM reporter_representations AS future
                   WHERE future.reporter_resource_id = candidate.reporter_resource_id
                     AND (future.generation > candidate.generation OR
                          (future.generation = candidate.generation AND
                           future.version > candidate.representation_version))
               ) THEN 'newer_history'
               ELSE NULL
           END AS exclusion_reason
    FROM age_candidates AS candidate
    LEFT JOIN resource AS root ON root.id = candidate.resource_id
    LEFT JOIN reporter_representations AS terminal
      ON terminal.reporter_resource_id = candidate.reporter_resource_id
     AND terminal.version = candidate.representation_version
     AND terminal.generation = candidate.generation
),
eligible_candidates AS (
    SELECT candidate.*,
           ROW_NUMBER() OVER (ORDER BY candidate.updated_at ASC, candidate.reporter_resource_id ASC) AS eligible_position
    FROM classified_candidates AS candidate
    WHERE candidate.exclusion_reason IS NULL
),
unknown_timestamp_candidates AS (
    SELECT rr.id AS reporter_resource_id, rr.resource_id,
           rr.representation_version, rr.generation
    FROM reporter_resources AS rr
    CROSS JOIN parameters AS p
    WHERE rr.tombstone IS TRUE
      AND rr.updated_at IS NULL
      AND (p.reporter_type_filter IS NULL OR rr.reporter_type = p.reporter_type_filter)
),
summary AS (
    SELECT
        p.evaluation_time_utc, p.cutoff_utc,
        COALESCE(p.reporter_type_filter, '') AS reporter_type_filter,
        p.bounded_row_limit,
        (SELECT COUNT(*) FROM classified_candidates) AS age_candidates,
        (SELECT COUNT(*) FROM unknown_timestamp_candidates) AS unknown_timestamp_rows,
        (SELECT COUNT(*) FROM eligible_candidates) AS eligible_reporter_resources,
        (SELECT COUNT(*) FROM eligible_candidates WHERE eligible_position <= p.bounded_row_limit) AS bounded_reporter_resources,
        (SELECT COUNT(*) FILTER (WHERE exclusion_reason = 'missing_root') FROM classified_candidates) AS excluded_missing_root,
        (SELECT COUNT(*) FILTER (WHERE exclusion_reason = 'root_type_mismatch') FROM classified_candidates) AS excluded_root_type_mismatch,
        (SELECT COUNT(*) FILTER (WHERE exclusion_reason = 'invalid_reporter_key') FROM classified_candidates) AS excluded_invalid_reporter_key,
        (SELECT COUNT(*) FILTER (WHERE exclusion_reason = 'missing_terminal_representation') FROM classified_candidates) AS excluded_missing_terminal_representation,
        (SELECT COUNT(*) FILTER (WHERE exclusion_reason = 'inconsistent_terminal_representation') FROM classified_candidates) AS excluded_inconsistent_terminal_representation,
        (SELECT COUNT(*) FILTER (WHERE exclusion_reason = 'newer_history') FROM classified_candidates) AS excluded_newer_history
    FROM parameters AS p
),
candidate_details AS (
    SELECT
        'age_candidate'::text AS record_type,
        candidate.reporter_resource_id, candidate.resource_id, candidate.updated_at,
        EXTRACT(EPOCH FROM (p.evaluation_time_utc - candidate.updated_at))::numeric(20,6) AS age_seconds,
        candidate.representation_version, candidate.generation, candidate.exclusion_reason,
        eligible.eligible_position,
        CASE WHEN eligible.eligible_position IS NOT NULL
                   AND eligible.eligible_position <= p.bounded_row_limit THEN 'true' ELSE 'false' END AS within_bounded_prefix
    FROM classified_candidates AS candidate
    CROSS JOIN parameters AS p
    LEFT JOIN eligible_candidates AS eligible USING (reporter_resource_id)

    UNION ALL

    SELECT
        'unknown_timestamp'::text,
        candidate.reporter_resource_id, candidate.resource_id, NULL::timestamptz,
        NULL::numeric(20,6), candidate.representation_version, candidate.generation,
        'missing_updated_at'::text, NULL::bigint, 'false'::text
    FROM unknown_timestamp_candidates AS candidate
),
artifact_rows AS (
    SELECT
        0 AS row_order, 'summary'::text AS record_type,
        summary.evaluation_time_utc, summary.cutoff_utc, summary.reporter_type_filter,
        summary.bounded_row_limit,
        NULL::uuid AS reporter_resource_id, NULL::uuid AS resource_id,
        NULL::timestamptz AS updated_at, NULL::numeric(20,6) AS age_seconds,
        NULL::bigint AS representation_version, NULL::bigint AS generation,
        NULL::text AS exclusion_reason, NULL::bigint AS eligible_position,
        NULL::text AS within_bounded_prefix,
        summary.age_candidates, summary.unknown_timestamp_rows,
        summary.eligible_reporter_resources, summary.bounded_reporter_resources,
        summary.excluded_missing_root, summary.excluded_root_type_mismatch,
        summary.excluded_invalid_reporter_key, summary.excluded_missing_terminal_representation,
        summary.excluded_inconsistent_terminal_representation, summary.excluded_newer_history
    FROM summary

    UNION ALL

    SELECT
        1, detail.record_type,
        p.evaluation_time_utc, p.cutoff_utc, COALESCE(p.reporter_type_filter, ''),
        p.bounded_row_limit,
        detail.reporter_resource_id, detail.resource_id, detail.updated_at, detail.age_seconds,
        detail.representation_version, detail.generation, detail.exclusion_reason,
        detail.eligible_position, detail.within_bounded_prefix,
        NULL::bigint, NULL::bigint, NULL::bigint, NULL::bigint,
        NULL::bigint, NULL::bigint, NULL::bigint, NULL::bigint,
        NULL::bigint, NULL::bigint
    FROM candidate_details AS detail
    CROSS JOIN parameters AS p
)
SELECT
    record_type, evaluation_time_utc, cutoff_utc, reporter_type_filter, bounded_row_limit,
    reporter_resource_id, resource_id, updated_at, age_seconds,
    representation_version, generation, exclusion_reason, eligible_position, within_bounded_prefix,
    age_candidates, unknown_timestamp_rows, eligible_reporter_resources, bounded_reporter_resources,
    excluded_missing_root, excluded_root_type_mismatch, excluded_invalid_reporter_key,
    excluded_missing_terminal_representation, excluded_inconsistent_terminal_representation, excluded_newer_history
FROM artifact_rows
ORDER BY row_order, eligible_position NULLS LAST, updated_at NULLS LAST, reporter_resource_id NULLS LAST;

COMMIT;
