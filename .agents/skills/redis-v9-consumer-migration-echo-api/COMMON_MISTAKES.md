# Common Mistakes

1. Starting Echo server before Redis init completes.
2. Changing middleware response contract during dependency upgrade.
3. Using broad retries on request path causing latency spikes.
4. Forgetting race tests for middleware that touches shared lock state.
