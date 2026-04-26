# Common Mistakes

1. Using default lock timeout for long-running batch jobs.
2. Changing lock key schema during migration and causing overlaps.
3. Treating lock contention as fatal instead of skipping current tick.
4. Rolling out without double-run simulation in staging.
