# Error Catalog

## 401 spikes after deploy

- Cause: middleware reading wrong Redis namespace or startup race.
- Fix: verify key naming, ensure Redis init completes before route serving.

## duplicate-request behavior changed

- Cause: idempotency middleware logic changed while migrating imports.
- Fix: pin behavior to previous status code/body contract and re-run regression tests.

## request timeouts increased

- Cause: Redis timeouts or retries too aggressive in request path.
- Fix: tune retries/delays for API path; keep fast-fail semantics for middleware checks.
