# Error Catalog

## duplicate cron execution

- Cause: lock key drift or lock TTL too short.
- Fix: standardize keys and increase TTL to cover max runtime.

## missed job windows

- Cause: excessive retries during lock acquisition or startup failure loops.
- Fix: tune retries and add clear scheduler health signals.

## lock expiry during job

- Cause: job runtime exceeds configured timeout.
- Fix: raise timeout or add safe lock extension strategy.
