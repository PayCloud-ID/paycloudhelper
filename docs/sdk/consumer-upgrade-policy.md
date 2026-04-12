# Consumer SDK Upgrade Policy

This policy defines how consumer services adopt paycloudhelper SDK updates safely.

## Cadence

- Dependency update cadence: weekly.
- Recommended automation: scheduled dependency PR for `bitbucket.org/paycloudid/paycloudhelper`.
- Emergency patch upgrades: immediate for security or production-impacting fixes.

## Version Pinning

- Prefer explicit tagged versions.
- Avoid floating branches in production services.
- Apply semantic version expectations:
  - PATCH: safe bugfixes.
  - MINOR: additive features.
  - MAJOR: coordinated migration required.

## Validation Rules For Upgrade PRs

1. Compile target service packages that import S3MinIO SDK.
2. Run service test scope related to file upload/download flows.
3. Run transport governance checks where available.
4. Verify no direct internal S3MinIO HTTP usage is introduced.

## Rollback Rule

If the upgrade PR fails CI or runtime smoke validation:

1. Revert to previous helper version.
2. Re-run failing test suite to confirm recovery.
3. Record issue in SDK stub change log and service incident notes.
4. Block rollout until compatibility fix is released.
