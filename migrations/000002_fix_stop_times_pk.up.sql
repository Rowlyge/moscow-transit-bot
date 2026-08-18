ALTER TABLE stop_times DROP CONSTRAINT stop_times_pkey;
ALTER TABLE stop_times RENAME COLUMN stop_times_id TO stop_times_id_raw;
ALTER TABLE stop_times ADD COLUMN global_id BIGINT PRIMARY KEY;
