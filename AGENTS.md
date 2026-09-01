# Project handoff: huddle

## Purpose

`huddle` is an experimental local Linux configuration manager centered on an
exact plan/review/apply contract. It explores whether Terraform-like saved
plans are useful for mutable host configuration without placing host resources
in Terraform state.

The prototype is intentionally small: manage files and systemd services on the
local machine. DNF/package management, SSH, inventories, arbitrary commands,
and plugins are explicitly out of the initial scope.

## Current state

- Commands:
  - `huddle plan -f CONFIG --out FILE`
  - `huddle show PLAN`
  - `huddle apply PLAN`
- Strict YAML configuration decoding.
- File resources manage exact content, mode, owner, and group.
- File plans show unified diffs and apply through a temporary file plus rename.
- systemd resources manage enabled and running state through `systemctl`.
- Services can explicitly reload or restart when referenced resources change.
- Dependencies and triggers are topologically ordered; cycles are rejected.
- Saved plans are versioned JSON and written with mode `0600` because desired
  file bytes may be sensitive.
- Apply rechecks each resource's observed starting state and rejects stale
  plans before changing that resource.
- Apply prompts unless `--yes` is supplied.

Example configuration lives in `examples/nginx.yaml`. Do not apply that example
to the host merely to test the program.

## Architecture

- `internal/model`: configuration and saved-plan data structures.
- `internal/config`: strict YAML loading, validation, reference checks, and
  topological ordering.
- `internal/engine`: file/systemd inspection, plan creation, diffing, stale
  checks, and apply.
- `internal/cli`: standard-library flag parsing, plan persistence, prompting,
  and human/JSON rendering.
- `internal/engine/stat_unix.go`: Unix ownership extraction.

Systemd is currently invoked through `systemctl`; file operations use Go's
native filesystem APIs.

## Verification

```bash
gofmt -w .
go test ./...
go vet ./...
go build -buildvcs=false -o huddle .
./huddle help
```

Tests use temporary files and cover file diff/apply, stale-plan rejection,
dependency ordering, and dependency-cycle rejection. There are no systemd
integration tests yet. Never mutate real systemd units or `/etc` files unless
the user explicitly asks for it.

## Plan semantics

- A file plan stores the exact desired bytes and observed existence, digest,
  mode, UID, and GID.
- File apply requires the complete observed state to still match.
- A systemd plan stores observed enabled/running state and desired operations.
- Systemd apply rechecks enabled/running state before executing actions.
- The plan records a digest of the YAML configuration, but apply currently does
  not reload or verify the source configuration. The plan artifact itself is
  the execution input, which is intentional for exact file bytes.

## Important limitations

- Applying multiple resources is not transactional. Earlier resources remain
  changed if a later resource is stale or fails.
- Preconditions are checked one resource at a time, not all at once before the
  first mutation.
- Plan artifacts are not signed or encrypted.
- Atomic file replacement does not yet preserve ACLs, extended attributes, or
  every SELinux edge case, and it does not fsync the file and parent directory.
- A rename replaces symlinks rather than following them; symlink policy is not
  yet explicit.
- systemd state is reduced to enabled/running booleans. Static, masked, failed,
  and activating states need richer modeling.
- No systemd daemon-reload operation is inferred for unit-file changes.
- No rollback, audit log, remote targets, directories, symlinks, templates,
  secrets integration, or package management.

## Good next steps

1. Add a preflight phase that validates every change before mutating anything.
2. Add systemd inspection/execution interfaces and hermetic unit tests with a
   fake implementation.
3. Model systemd load, enablement, active, and failed states precisely.
4. Harden file replacement with fsync and explicit symlink/SELinux behavior.
5. Exercise `huddle` on one noncritical, purpose-built test service and use the
   experience to improve plan readability before expanding scope.

Preserve the narrow resource set and exact-plan thesis until real usage proves
which additional feature is necessary.
