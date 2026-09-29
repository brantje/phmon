CREATE INDEX mob_observation_samples_heatmap_time_idx
    ON mob_observation_samples (lower(server_name), dataset_id, sampled_at DESC);
