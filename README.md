# huddle

`huddle` is an experimental, local-only Linux configuration planner. Its first
prototype manages files and systemd services with saved plans and stale-state
checks.

```bash
go build -o huddle .
sudo ./huddle plan -f examples/nginx.yaml --out nginx.plan.json
./huddle show nginx.plan.json
sudo ./huddle apply nginx.plan.json
```

Plans are written with mode `0600` because they contain the exact desired file
content. Treat them as potentially sensitive artifacts.

## Prototype scope

- YAML configuration
- File content, mode, owner, and group
- Unified file diffs
- Atomic file replacement
- systemd enable/disable and start/stop
- Explicit reload/restart triggers
- Saved JSON plans
- Stale file and service-state rejection

Not yet implemented: remote hosts, inventories, packages, directories,
symlinks, secrets, signatures, rollback, or a plugin system.
