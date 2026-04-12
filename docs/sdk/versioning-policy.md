# S3MinIO SDK Versioning Policy

Use semantic versioning for shared helper releases.

## Version Bumps
- PATCH: bug fixes without API or behavior changes.
- MINOR: backward-compatible additions in contracts or adapters.
- MAJOR: breaking changes to exported contracts or payload semantics.

## Compatibility
- Keep previous adapter behavior stable during migration windows.
- Announce deprecations one minor release before removal.
- Coordinate consumer rollout when introducing required contract fields.
- Enforce service-scoped imports (`sdk/services/<service>`) through CI governance scripts.
