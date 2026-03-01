# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

- **Unit tests**
  - Root package: `LockError` (`Error`, `Unwrap`), Redis options (`InitRedisOptions`, `GetTrxRedisBackoff`, `GetTrxRedisLockTimeout`, `GetRedisPoolClient` when not initialized), mutex map (`StoreMutex`, `GetMutex`, `RemoveMutex`), init/app env (`SetAppName`, `SetAppEnv`, `GetAppName`, `GetAppEnv`, `InitializeApp`), validator constants and header validation (idem key, CSRF), `LoggerErrorHub`.
  - `phhelper`: globenv (Get/Set app name and env), helpers (`JsonMinify`, `JsonMarshalNoEsc`, `JSONEncode`, `ToJson`, `ToJsonIndent`).
  - `phjson`: config, `Marshal`, `Unmarshal`, `MarshalIndent`, invalid JSON handling.
- **Scripts**
  - `scripts/run_tests.sh` — run all tests from repo root with options: `-v`, `-race`, `-cover`, `-coverprofile`, `-short`, `-h`.
- **CI**
  - Bitbucket Pipelines (`bitbucket-pipelines.yml`): on every push to `develop` and `main` (and default branches), run `go build ./...`, `go vet ./...`, `go test ./...`. Pipeline fails if any step fails.
- **Documentation**
  - README: **Testing** section (run script, options, coverage, code quality); **Verifying the library** (build/vet/test checklist); **CI (Bitbucket Pipelines)** (how CI runs and how to require passing pipeline for merges).

### Changed

- None.

### Fixed

- None.

### Security

- None.

---

## [1.8.0] and earlier

History before this changelog was introduced. See git tags and release notes for older versions.

Retracted versions (do not use): v1.6.3, v1.6.0, v1.5.2 — see `go.mod` retract block.

[Unreleased]: https://bitbucket.org/paycloudid/paycloudhelper/compare/v1.8.0..HEAD
[1.8.0]: https://bitbucket.org/paycloudid/paycloudhelper/src/v1.8.0
