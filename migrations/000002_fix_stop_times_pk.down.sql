ALTER TABLE stop_times DROP COLUMN global_id;
ALTER TABLE stop_times RENAME COLUMN stop_times_id_raw TO stop_times_id;
ALTER TABLE stop_times ADD PRIMARY KEY (stop_times_id);
