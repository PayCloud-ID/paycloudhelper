# SDK Scaffold Pattern

Use this pattern when onboarding another service into the shared SDK layout.

## Standard layout
- `sdk/services/<service>/helper`
- `sdk/services/<service>/grpc`
- `sdk/services/<service>/http`
- `sdk/services/<service>/pb`
- `sdk/services/<service>/proto`
- `sdk/services/<service>/facade`

## Phase model
- Phase 1: create wrappers and docs first, keep legacy package paths working.
- Phase 2: move implementation ownership into the service-scoped packages.
- Phase 3: add service-specific proto automation and CI gates.

## Generator
Run `make proto.service.scaffold SERVICE=<service>` to create the baseline directories and starter docs.
