# Script Documentation Standard

## Required Header for Every Script Change

Whenever an agent creates or updates a shell script (`*.sh`), it must add or update a top-of-file documentation header directly below the shebang.

The header must include all sections below:

1. `Purpose`
2. `Usage`
3. `Options` (or explicitly state no options)
4. `What It Reads`
5. `What It Affects / Does`
6. `Exit Behavior` (recommended; required for guard/check scripts)

## Required Format

Use concise comment blocks with consistent labels:

```bash
#!/usr/bin/env bash
# script-name.sh
#
# Purpose:
# - ...
#
# Usage:
# - ./path/to/script.sh [options]
#
# Options:
# - --flag  Description
#
# What It Reads:
# - Files, env vars, tools, and inputs consumed by this script.
#
# What It Affects / Does:
# - Files/directories modified, commands executed, side effects.
#
# Exit Behavior:
# - 0 on success, non-zero on failure conditions.
```

## Scope and Intent

- Applies to root scripts and nested scripts under directories like `scripts/`.
- Do not change runtime behavior only to satisfy documentation requirements.
- Keep headers synchronized with actual script behavior whenever logic changes.
- If a script delegates with `exec`, document delegation clearly in `Purpose` and `What It Affects / Does`.

## Validation Checklist

- Header exists below shebang.
- All required sections are present and accurate.
- Usage examples reflect real arguments and defaults.
- Read/affect sections mention important file paths and env variables.
- Documentation is updated in the same change as script logic edits.
