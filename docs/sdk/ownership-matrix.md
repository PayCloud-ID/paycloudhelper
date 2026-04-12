# SDK Ownership Matrix

This document defines ownership and review responsibilities for service-scoped SDK packages.

## Current Matrix

| SDK | Owner Team | Required Reviewers | Oncall | Notes |
|---|---|---|---|---|
| `sdk/services/s3minio` | Platform Shared Libraries | S3MinIO Provider + Platform Shared Libraries | Shared Libraries Oncall | Canonical reference SDK for transport/governance patterns. |

## Required PR Gates For SDK Changes

1. Owner review required.
2. Provider team review required when proto or transport behavior changes.
3. Documentation sync required: README, AGENTS/Copilot bridge, docs/sdk artifacts.
4. Governance checks must pass (`ci.check.direct-http`, `ci.check.stub-drift`).

## Expansion Rule

For each new service SDK under `sdk/services/<service>`:

1. Add an ownership row here.
2. Add service onboarding notes and proto lifecycle references.
3. Add dependency upgrade guidance for known consumers.
