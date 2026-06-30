# v-rpc-debug docs

The `v rpc` domain — a debug/validation tool that taps the RPC Broker's native
`XWBDEBUG` log over the m-driver-sdk seam (`v rpc-debug`), plus the CPRS↔VistA
network helpers (`v rpc-debug doctor` / `v rpc-debug relay`).

## Key docs

- [v-rpc-debug users guide](v-rpc-debug-users-guide.md) — how to view and save live RPC
  traffic with `v rpc-debug`, and connect CPRS to vehu (`doctor`/`relay`). **Current
  guide** (supersedes the older `v-rpc-user-guide.md`, now in `archive/`).

## Folders

- `proposals/` — design proposals for this domain ([durable S3 tap (moved to the
  central docs repo)](proposals/v-rpc-tap-durable-s3.md)).
- `memory/` — per-repo auto-memory (durable lessons; see [MEMORY.md](memory/MEMORY.md)).
- `archive/` — retired docs: the implemented [implementation plan](archive/v-rpc-implementation-plan.md)
  + [network-doctor proposal](archive/v-rpc-network-doctor.md), and the superseded
  [older user guide](archive/v-rpc-user-guide.md).
