# Documentation & Changelog — paycloudhelper

## When to Update

| Change type | README.md | CHANGELOG.md |
|-------------|-----------|--------------|
| New public API, config, or script | Yes | Yes (under **Added**) |
| Changed behavior or deprecation | Yes | Yes (**Changed** / **Deprecated**) |
| Bug fix (user-visible) | If it affects docs | Yes (**Fixed**) |
| Security fix | If it affects docs | Yes (**Security**) |
| Removed API or option | Yes | Yes (**Removed**) |
| Typo, internal refactor, no user impact | Only if doc was wrong | Optional |

## README.md

- Update when you add or change anything **user-facing**: APIs, env vars, configuration, scripts, workflows, or package structure.
- Keep **Quick Start**, **API Reference**, and **Configuration** in sync with code.
- If you add a new capability (e.g. a script, a section like **Testing**), add or update the relevant section and the Table of Contents.

## CHANGELOG.md

- Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
- All release-worthy changes go under **`[Unreleased]`** in the right category:
  - **Added** — new features, APIs, scripts, docs sections.
  - **Changed** — behavior or compatibility changes.
  - **Deprecated** — soon-to-be-removed APIs.
  - **Removed** — removed APIs or options.
  - **Fixed** — bug fixes.
  - **Security** — security-related fixes.
- When releasing a new version:
  1. Replace `[Unreleased]` with the new version heading (e.g. `## [1.9.0]`).
  2. Add a new `## [Unreleased]` section at the top.
  3. Add comparison links at the bottom (e.g. `[1.9.0]: .../compare/v1.8.0...v1.9.0`).

## Agent Rule

**Every time you make a change that affects users or the public API, update README.md and/or CHANGELOG.md as part of the same change.** Do not leave documentation updates for "later."
