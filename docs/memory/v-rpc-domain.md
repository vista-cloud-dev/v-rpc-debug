---
name: v-rpc-domain
description: v-rpc is a new Go `v` domain (`v rpc debug`) that taps the RPC Broker's native XWBDEBUG log over the m-driver-sdk seam to view/save live RPC traffic for offline comparison against the VSL tap. Built 2026-06-26; v-cli mount deferred until the repo is published+tagged.
metadata:
  type: project
---

**v-rpc created 2026-06-26.** A new repo under vista-cloud-dev: the `v rpc`
domain, Go, exports importable `rpccli` (the `v` umbrella will mount it as
`v rpc`, mirroring v-pkg/pkgcli). Layer `v`. Headline capability **`v rpc debug`**
taps the RPC Broker's *native* `XWBDEBUG` log (`^XTMP("XWBLOG"_$J)`) over the m
engine driver seam to **view live RPC traffic in the terminal** or **save it to a
file as LDJSON** for **offline comparison against the Phase-2 VSL tap** — a
debug/validation tool, NOT a durable egress tap (that's the VSL hook).

**Locked design (with owner):** `v rpc debug {tail,capture,status,arm,disarm}`;
shared flags `--all/--filter/--interval/--duration/--level{2,3}/--keep/--no-clear`;
explicit `--engine ydb|iris` (ydb/vehu now, IRIS-VistA for VA validation later);
capture LDJSON field names align with the s3tap envelope (`rpc`,`ts`,`job`,`seq`)
so the two captures can be **joined offline and separately** — correlation is NOT
in this tool. Level 3 logs params = PHI (default 2 = names only). CLI viewer now;
TUI later.

**Architecture (waterline-clean):** `internal/xwblog` = pure parse/record/LDJSON/
dedup (no engine dep, TDD); `internal/capture` = arm/disarm + poll + dedup over a
small `Execer` interface (fake-tested); `rpccli` = clikit (kong) command surface
adapting `mdriver.Client` to `Execer`; `main.go` = standalone binary. Engine
access ONLY via `mdriver.Client` (waterline rule 3), never raw `docker exec`. The
per-$J LOGSTART wipe makes XWBLOG lossy for complete capture — documented; fine
for the oracle role. See the vehu-side mechanics in the shared
`cprs-rpc-xwbdebug-host-probe` memory.

**State:** `make check` green (gofmt+golangci-lint+race+build); `internal/*`
74–80% covered; live `status` proven against vehu through the real driver. Deps
pin clikit v0.1.0 + m-driver-sdk v0.3.0 (airgapped, no `replace`). Furniture from
v-pkg/go-cli-template.

**OWED (owner):**
1. `gh repo create vista-cloud-dev/v-rpc` + push `main` (repo creation is the
   owner's step per org convention).
2. Run the live `arm`/`capture`/`tail` streaming validation (state-changing engine
   ops — held back this session); confirm restore-on-exit leaves `XWBDEBUG=1`.
3. **I5 (deferred): mount into v-cli** — add `vcontract.Contract()` to `rpccli`
   (mirror pkgcli/contract.go), then in v-cli add `Rpc rpccli.Commands` +
   `rpccli.Contract()` to the registry. Needs v-rpc published + tagged (v-cli pins
   versions, no `replace`).
