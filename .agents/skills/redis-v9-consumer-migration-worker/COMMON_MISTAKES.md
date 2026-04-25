# Common Mistakes

1. Logging contention as errors and alerting unnecessarily.
2. Forgetting to release lock in early-return branches.
3. Removing lock key prefixes and breaking cross-service coordination.
4. Skipping queue + Redis integration smoke tests before production rollout.
