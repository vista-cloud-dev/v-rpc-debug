---
title: v-rpc-debug users guide — viewing and saving live RPC traffic with `v rpc-debug`
status: draft
version: v0.3.0
created: 2026-06-30
last_modified: 2026-06-30
doc_type: [GUIDE]
layer: v
supersedes: v-rpc-user-guide.md
---

# v-rpc-debug users guide

> **Supersedes `v-rpc-user-guide.md`.** This is the current, canonical guide for
> `v-rpc-debug` / `v rpc-debug`. The older `v-rpc-user-guide.md` is retained only
> for history.

## Contents

1. [Introduction and background](#1-introduction-and-background)
2. [The need for this tool](#2-the-need-for-this-tool)
3. [Features and functions](#3-features-and-functions)
4. [Command summary (what each does + when to use it)](#4-command-summary)
5. [Setup (one time)](#5-setup-one-time)
6. [Selecting the engine](#6-selecting-the-engine)
7. [Logging into vehu — credentials, user, and the richest patient](#7-logging-into-vehu)
8. [Common flags](#8-common-flags)
9. [End-to-end runs (copy/paste)](#9-end-to-end-runs)
10. [Connecting CPRS to vehu (networking)](#10-connecting-cprs-to-vehu-networking)
11. [Comparing against the VSL tap](#11-comparing-against-the-vsl-tap)
12. [Limitations](#12-limitations)
13. [Troubleshooting](#13-troubleshooting)
14. [References](#14-references)

---

## 1. Introduction and background

`v-rpc-debug` (mounted in the `v` umbrella as **`v rpc-debug`**) shows you the
**RPCs flowing through a VistA RPC Broker**, live in your terminal, and can save
them to a file for later analysis.

VistA's clients — most visibly **CPRS** — never touch the M database directly.
They speak to the **RPC Broker** (the `XWB` package), which dispatches each request
to a named **Remote Procedure Call (RPC)** that runs M code on the engine and
returns a result. A single CPRS sign-on and chart open fires **hundreds** of these
RPCs in a precise sequence. That traffic is normally invisible: it happens on a TCP
socket between a Windows client and the engine, and nothing writes it down.

`v-rpc-debug` makes that traffic visible **without installing or patching
anything** on the engine. It does this with the Broker's **own, built-in** debug
facility, **`XWBDEBUG`**. When raised to debug level, the Broker writes each RPC it
dispatches to a scratch global (`^XTMP("XWBLOG"_$J)`). `v-rpc-debug` toggles that
level, reads the log over the **m-driver-sdk engine seam**, parses it, and streams
or saves it — then restores the Broker to its prior state on exit. Turning the tap
on and off is a single parameter toggle that the tool manages for you.

Everything the tool does to the engine goes **only** through the `mdriver.Client`
seam (the m-driver-sdk → `m-ydb`/`m-iris` driver stack) — never raw `docker exec`,
never a hand-rolled M interpreter call. That is the org's mandatory engine-access
rule, and it means the same binary works against YottaDB (`vehu`) and IRIS-VistA
(`foia*`) with only an `--engine` change.

**What it is — and is not.** `v-rpc-debug` is a **debug / validation** tool, not a
durable capture pipeline:

- `XWBDEBUG` logs RPC **names** — not results, not request↔response correlation,
  not (at the safe level) parameters.
- It can **drop traffic** under load (the log is per-job and wiped at each new
  connection — see [Limitations](#12-limitations)).
- It has **no S3 / egress**.

The durable, complete, egress-capable capture path is the separate **VSL broker
hook** (the Phase-2 tap). `v-rpc-debug` is the **zero-install oracle** you check
that tap against: it produces LDJSON whose field names (`rpc`, `ts`, `job`, `seq`)
deliberately line up with the s3tap envelope, so the two captures can be joined and
diffed **offline**.

---

## 2. The need for this tool

Three recurring questions motivated `v-rpc-debug`, and each maps to a use case it
answers directly:

1. **"Is the client actually reaching the Broker, and what is it calling?"**
   When CPRS (or any Broker client) misbehaves, the first thing you need is ground
   truth: *which* RPCs fire, *in what order*. Before this tool that meant guessing
   from CPRS-side symptoms. Now you `tail` the Broker and watch the real sequence —
   e.g. confirm the canonical sign-on chain `XUS SIGNON SETUP → XUS INTRO MSG →
   XUS AV CODE → XUS GET USER INFO → XWB CREATE CONTEXT …` before chart-load RPCs.

2. **"Did my VSL tap capture the same RPCs the Broker really saw?"**
   The VSL hook is the durable capture, but a capture you can't validate is a
   liability. `XWBDEBUG` is the Broker's *own* accounting of what it dispatched, so
   it is the **independent oracle**: capture the same workload both ways and diff
   the `rpc` sets offline. Anything the oracle saw that the tap missed is a tap bug.

3. **"Why won't CPRS even connect?"**
   The single most error-prone step in a VistA-in-Docker + CPRS-in-a-VM lab is the
   network path — it fails with the same opaque `WSAECONNREFUSED`. `v-rpc-debug`
   folds in a **network doctor** and a **built-in relay** so the same tool that
   reads the traffic also gets the client talking to the engine in the first place.

The design constraints that follow from this role:

- **Zero install on the engine** — uses a facility VistA already ships; you can run
  it against a production-fidelity image and leave it byte-clean.
- **Engine-portable** — one binary, `--engine ydb|iris`, via the driver seam.
- **Safe by default** — logs RPC **names only** unless you explicitly opt into the
  PHI-bearing parameter level on a test system.
- **Comparable output** — LDJSON aligned with the s3tap envelope so the oracle
  role actually works.

---

## 3. Features and functions

| Capability | What it gives you |
|---|---|
| **Live viewer** (`tail`) | Watch RPC names stream into your terminal in real time as a client drives the Broker; restores the Broker level on exit. |
| **File capture** (`capture`) | Append each RPC to a file as LDJSON (one JSON object per line) for offline analysis and tap comparison. |
| **Zero-install tap** | Uses the Broker's native `XWBDEBUG` — nothing is added to or patched on the engine; the tool only toggles a parameter and reads a scratch global. |
| **Engine-portable** | Same binary for YottaDB (`vehu`) and IRIS-VistA (`foia*`) via `--engine`, all through the `mdriver.Client` seam — never raw `docker exec`. |
| **State management** | `status`/`arm`/`disarm`/`clear` give explicit control of the debug level and the buffered log; `tail`/`capture` arm + restore automatically. |
| **Self-test traffic** (`ping`) | Fire harmless no-arg RPCs straight at the Broker port so a tap has something to capture — no CPRS or external client needed. |
| **Network doctor** (`doctor`) | Walk the whole CPRS→VistA path (docker → publish mode → listener → relay), pinpoint the broken hop, and print the exact address to type into CPRS. |
| **Built-in relay** (`relay`) | A dependency-free TCP forwarder (no `socat`) that republishes a loopback-bound Broker on a VM-reachable interface; optional persistent `systemd --user` service. |
| **Safe-by-default capture** | Logs RPC **names only** (level 2); the parameter-bearing level 3 is PHI and must be opted into explicitly on a test system. |
| **Comparable output** | LDJSON field names (`rpc`, `ts`, `job`, `seq`) align with the s3tap envelope so oracle-vs-tap diffs are a straight set comparison. |
| **Flexible filtering** | `--all`, `--filter TEXT`, `--interval`, `--duration` shape what you see and how long you watch. |
| **Discovery surface** | `menu` (interactive palette), `version`, `schema` (machine-readable CLI contract), `install-completions` — none of which touch an engine. |
| **Flagless operation** | Engine knobs read env vars (`VRPC_ENGINE`/`VRPC_TRANSPORT`/`VRPC_CONTAINER`); set them once (direnv) and run `v-rpc-debug status` with no flags. |

Once mounted into the `v` umbrella, every command is available as `v rpc-debug …`.
Standalone, it is `v-rpc-debug …` — the form used throughout this guide.

---

## 4. Command summary

The surface has three groups: **Capture** (the `XWBDEBUG` tap), **Connect** (get
CPRS talking to VistA), and **Discovery** (describe the tool; no engine needed).

### Capture commands (reach the engine via the driver seam)

| Command | What it does | Use it when… |
|---|---|---|
| `status` | Read-only; shows the current `XWBDEBUG` level and how many log jobs / RPC lines are buffered. | You want to check state before/after a run, or confirm the level restored to 1 (stock). |
| `tail` | Arms `XWBDEBUG`, clears the log, streams new `RPC:` lines to the terminal until Ctrl-C, then **restores the prior level**. | You want to *watch* live traffic — confirm a client is reaching the Broker and see the RPC sequence. |
| `capture` | Same as `tail`, but appends each record to a file as LDJSON (echoes to stderr unless `--quiet`). | You want to *save* a workload (e.g. a CPRS login) for offline analysis or tap comparison. |
| `arm` | Sets `XWBDEBUG` to level 2 (names) and leaves it on. | You want the tap on across several manual steps (arm → drive a client by hand → `capture --no-clear`). |
| `disarm` | Restores `XWBDEBUG` to level 1 (stock). | A run left the level armed (e.g. overlapping runs, or a hard kill) and you want stock again. |
| `clear` | Wipes all `^XTMP("XWBLOG"*)` nodes so the engine is pristine; reports lines removed. | You want a clean slate before a run, or to tidy up after (the buffer otherwise auto-purges in ~7 days). |
| `ping` | Fires harmless no-arg RPCs straight at the Broker's TCP port (RPC-client role; takes `--addr`, **not** the engine flags). | You need traffic to test a `tail`/`capture` without launching CPRS or any external client. |

### Connect commands (host-side networking — no engine writes)

| Command | What it does | Use it when… |
|---|---|---|
| `doctor` | Walks the CPRS→VistA path (docker → publish mode → Broker listener → relay), flags the broken hop, and prints the exact `s=… p=…` address for CPRS. `--fix` starts the relay if it's needed and missing. | CPRS can't connect (`WSAECONNREFUSED`), or before a CPRS run to confirm the path is green. |
| `relay` | A built-in TCP forwarder (no `socat`) that republishes the loopback-bound Broker on `0.0.0.0:19431` so a VM can reach it. `--install`/`--uninstall`/`--status` manage a persistent `systemd --user` service. | The Broker is published loopback-only and a VM/CPRS must reach it; or you want the relay to survive reboots. |

### Discovery commands (describe the tool; work with no driver/container/Docker)

| Command | What it does | Use it when… |
|---|---|---|
| `menu` | Opens a full-screen, keyboard-driven palette over the command tree (breadcrumb, categories, per-command detail + status badge). Non-TTY: prints styled help. | You want to browse the surface without memorizing flags. |
| `version` | Shows version and build info. | Reporting an issue, or confirming which build you're running. |
| `schema` | Emits the entire command/flag/enum tree as JSON (hidden from `--help`). | Scripting, or an agent discovering the CLI contract. |
| `install-completions` | Installs shell tab-completion (hidden from `--help`). | One-time shell setup. |

---

## 5. Setup (one time)

`v-rpc-debug` is a single static binary. It reaches the engine **only** through the
m-driver-sdk seam, so it needs the `m-<engine>` driver binary reachable — the one
external dependency. Put both binaries on your `PATH` **in the same directory** and
`v-rpc-debug` locates the driver automatically (driver-contract §4) — **no
`M_<ENGINE>_BIN` to set**:

```bash
cd ~/vista-cloud-dev/v-rpc-debug && make install BINDIR=~/scripts/bin   # installs v-rpc-debug
install -m755 ~/vista-cloud-dev/m-ydb/dist/m-ydb ~/scripts/bin/         # co-locate the driver
```

The only thing `v-rpc-debug` can't guess is **which engine container** to talk to.
Set it once — engine defaults to `ydb`, transport to `docker`:

```bash
export VRPC_CONTAINER=vehu        # once per shell, or in the repo .envrc (direnv)
```

That's the whole configuration. Everything is now flagless:

```bash
v-rpc-debug status
v-rpc-debug tail
v-rpc-debug capture --out rpc.ldjson
```

**Requirements:** Docker running with the target engine container up (e.g. `vehu`).
For IRIS, co-locate the `m-iris` driver and add `--engine iris` (or
`VRPC_ENGINE=iris`). `M_<ENGINE>_BIN` still works as an explicit override if you
keep the driver somewhere else.

---

## 6. Selecting the engine

Engine-side commands take three knobs, all with sensible defaults so you usually
override **nothing** (just set the container once, per §5):

| Flag | Default | Env | Meaning |
|---|---|---|---|
| `--engine` | `ydb` | `VRPC_ENGINE` | which engine: `ydb` or `iris` (IRIS = explicit opt-in for VA validation) |
| `--transport` | `docker` | `VRPC_TRANSPORT` | driver transport: `local`, `docker`, `remote` |
| `--container` | — | `VRPC_CONTAINER` | container/instance name; sets `M_<ENGINE>_CONTAINER` |

```bash
v-rpc-debug status                                    # ydb / docker / $VRPC_CONTAINER
v-rpc-debug status --engine iris --container foia-t12 # override ad hoc
```

(The connection — container, base URL, credentials — is otherwise read by the
driver from its `M_<ENGINE>_*` environment, exactly like `m vista`.)

### Setting defaults (skip the flags)

Every engine flag also reads an environment variable, so set them once and drop the
flags entirely:

| Flag | Env var |
|---|---|
| `--engine` | `VRPC_ENGINE` |
| `--transport` | `VRPC_TRANSPORT` |
| `--container` | `VRPC_CONTAINER` |
| `--addr` (`ping`) | `VRPC_ADDR` |
| `--listen` / `--relay-addr` (`relay`/`doctor`) | `VRPC_RELAY_ADDR` |

```bash
export M_YDB_BIN=~/vista-cloud-dev/m-ydb/dist/m-ydb     # driver location (or co-locate, per §5)
export VRPC_ENGINE=ydb VRPC_TRANSPORT=docker VRPC_CONTAINER=vehu

v-rpc-debug status        # no --engine/--transport/--container needed
v-rpc-debug tail
v-rpc-debug capture --out rpc.ldjson
```

A flag on the command line always **overrides** its env var, so you can keep ydb as
the default and switch to IRIS ad hoc with `--engine iris`.

**Make it persistent** with [direnv](https://direnv.net) — the house per-project
env backbone. Add to `~/vista-cloud-dev/v-rpc-debug/.envrc` (then `direnv allow`):

```bash
export M_YDB_BIN="$HOME/vista-cloud-dev/m-ydb/dist/m-ydb"
export VRPC_ENGINE=ydb VRPC_TRANSPORT=docker VRPC_CONTAINER=vehu
```

so the defaults apply automatically whenever you `cd` into the repo. (Or put the
same `export`s in your shell rc for a machine-wide default.) The driver-native
`M_<ENGINE>_CONTAINER` (e.g. `M_YDB_CONTAINER=vehu`) also works for the container if
you prefer the driver's own variable.

---

## 7. Logging into vehu

To drive real traffic you log a client (CPRS) into VistA. For the `worldvista/vehu`
demo image these access/verify codes are documented and **confirmed against this
engine** (access-code hashes resolve to the named `#200` users — see
[References](#14-references)):

| User | Access | Verify | Role |
|---|---|---|---|
| **PROVIDER,VERO** | `CAS123` | `CAS123..` | **provider — recommended** |
| VEHU,TEN | `10VEHU` | `QXYG~011` | demo clinician |
| System Manager | `PRO1234` | `PRO1234!!` | admin / programmer |

E-signature code (to sign orders/notes): `123456`.

**Recommended user type → a clinical *provider*** (e.g. **PROVIDER,VERO** /
`CAS123`). A provider sees the full chart *and* can place orders, so a session
exercises the widest set of RPCs (orders, notes, consults, meds, labs, vitals) —
the richest traffic for tap validation. The System Manager account can browse but
isn't a clinician, so it fires a narrower set.

At the CPRS sign-on screen type the access code, Tab, then the verify code — or type
`CAS123;CAS123..` in the Access field. (On these demo images you may be prompted to
choose a new verify code on first login; just set one.)

### The patient with the most data

Picked from a live read-only probe of vehu (order/note/visit cross-references, see
[References](#14-references)). For the **richest, most varied** chart — the best
choice for exercising the most RPCs across tabs — open:

> **TEN,PATIENT** — **691 notes, 190 visits, 236 orders** (by far the most clinical
> documents and encounters of any patient). Search `TEN,PATIENT` in CPRS.

Runners-up with broad data: **FORTYSIX,PATIENT** (48 notes / 121 visits / 175
orders) and **THIRTYSIX,PATIENT** (64 / 68 / 170).

If you specifically want **maximum orders** (and nothing else), **SEVENTYEIGHT,
OUTPATIENT** has 1,508 orders — but **0 notes and 0 visits** (it's an order-load
patient), so it lights up far fewer tabs than TEN,PATIENT. For a representative
full-chart RPC sweep, prefer **TEN,PATIENT**.

---

## 8. Common flags

For `tail` and `capture`:

| Flag | Default | What it does |
|---|---|---|
| `--all` | off | Show every log line (connect/reject/etc.), not just `RPC:` lines |
| `--filter TEXT` | — | Only RPCs whose name contains `TEXT` (case-insensitive) |
| `--interval N` | `1.0` | Poll the log every `N` seconds |
| `--duration D` | `0` | Stop after `D` (e.g. `30s`, `5m`); `0` = run until Ctrl-C |
| `--level {2,3}` | `2` | Debug level to arm — **see the PHI warning below** |
| `--keep` | off | Leave `XWBDEBUG` armed on exit instead of restoring |
| `--no-clear` | off | Don't wipe the existing log on start (capture what's already buffered) |
| `--restore-to N` | `-1` | Level to restore on exit (`-1` = the level found at start). Use `1` to force stock if a prior/overlapping run left it armed |
| `--out PATH` | — | (`capture` only, required) output file; `file://PATH` or `PATH` |
| `--quiet` | off | (`capture` only) don't echo to the terminal |

### Output format & global flags

Every command honors the shared clikit globals:

| Flag | Meaning |
|---|---|
| `-o`, `--output {text,json,auto}` | `text` (styled, default on a TTY), `json` (machine-readable), or `auto` |
| `--no-color` | Disable ANSI styling even on a TTY |
| `-v`, `--verbose` | Verbose diagnostics to stderr |

For `tail`, `-o json` streams the same LDJSON records `capture` writes — handy for
piping into `jq`:

```bash
v-rpc-debug tail --engine ydb --container vehu -o json | jq -r .rpc
```

### ⚠️ PHI warning — level 2 vs 3

- **Level 2 (default)** logs RPC **names only** — safe.
- **Level 3** logs the full call string **including RPC parameters**, which on a
  real VistA is **PHI**, written in cleartext to `^XTMP`. Only use `--level 3` on a
  test system with no real patient data, and clear the log afterward.

---

## 9. End-to-end runs

### A. Self-contained (no CPRS) — ping the live broker, examine, clean up

Copy/paste the whole block; it pings vehu's broker, saves the captured RPCs, shows
them, then leaves the engine pristine (level 1, log cleared):

```bash
export M_YDB_BIN=~/vista-cloud-dev/m-ydb/dist/m-ydb
ENG="--engine ydb --transport docker --container vehu"

v-rpc-debug status $ENG                                   # 1. baseline (level 1, off)
v-rpc-debug arm    $ENG                                   # 2. arm capture (level 2)
v-rpc-debug ping   --addr 127.0.0.1:9430                  # 3. fire real RPCs at the broker
v-rpc-debug capture $ENG --out smoke.ldjson \
      --no-clear --duration 2s --restore-to 1            # 4. save buffered RPCs -> file, restore to 1

# 5. examine the output
cat smoke.ldjson
jq -r '.rpc' smoke.ldjson                                 # just the RPC names
jq -r '"\(.seq)\t\(.rpc)"' smoke.ldjson                   # seq + name, in order
jq -s 'group_by(.rpc)|map({rpc:.[0].rpc,n:length})|sort_by(-.n)' smoke.ldjson  # counts

v-rpc-debug clear  $ENG                                   # 6. wipe the buffered log
v-rpc-debug status $ENG                                   # 7. confirm: level 1 (off), 0 buffered
```

Each captured line looks like:

```json
{"source":"xwbdebug","schema_version":1,"kind":"rpc","rpc":"XWB IM HERE","ts":"67747,48090","job":416,"seq":4,"msg":"RPC: XWB IM HERE"}
```

### B. Real CPRS — capture a live login + chart sweep

CPRS runs in the VM; `v-rpc-debug` runs on the host. **Before launching CPRS, make
sure the network path is healthy** — run `v rpc-debug doctor` and, if it says so,
start the relay (see [Connecting CPRS to vehu](#10-connecting-cprs-to-vehu-networking)
below). With the path green, CPRS connects to the address `doctor` prints (vehu:
`s=10.0.2.2 p=19431`).

```bash
export M_YDB_BIN=~/vista-cloud-dev/m-ydb/dist/m-ydb
ENG="--engine ydb --transport docker --container vehu"

# 1. start capturing (arms, clears, streams to file). Leave it running.
v-rpc-debug capture $ENG --out cprs-login.ldjson --restore-to 1

#   --- in the VM: launch CPRS, sign in as PROVIDER,VERO (CAS123;CAS123..),
#       open patient TEN,PATIENT, click through the chart tabs ---

# 2. back on the host: Ctrl-C the capture  (restores XWBDEBUG to 1)

# 3. examine
wc -l cprs-login.ldjson
jq -r '"\(.seq)\t\(.rpc)"' cprs-login.ldjson | head -20         # the sign-on sequence
jq -s 'group_by(.rpc)|map({rpc:.[0].rpc,n:length})|sort_by(-.n)|.[:20]' cprs-login.ldjson

# 4. clean up
v-rpc-debug clear  $ENG
v-rpc-debug status $ENG                                   # level 1 (off), 0 buffered
```

> A complete real CPRS sign-on captured this way runs to ~1,120 RPC records / ~242
> distinct RPCs across the connection — the canonical chain begins `XUS SIGNON SETUP
> → XUS INTRO MSG → XUS AV CODE → XUS GET USER INFO → XWB GET BROKER INFO → XUS
> DIVISION GET → XWB CREATE CONTEXT …` before chart-load RPCs. Capture files
> (`*.ldjson`) are git-ignored — they're data, not source.

---

## 10. Connecting CPRS to vehu (networking)

CPRS-in-a-VM reaching VistA-in-Docker is the single most error-prone step, and it
always fails the same opaque way: CPRS shows **`WSAECONNREFUSED / WASConnectByName`**.
The cause is almost always that the broker is published to the host's **loopback
only** (Docker `127.0.0.1:9430`), which a VM cannot reach — so the path needs a
relay on a reachable interface. `v rpc-debug doctor` diagnoses the whole chain and
tells you exactly what to do; `v rpc-debug relay` is the fix.

```bash
v-rpc-debug doctor                     # walk docker -> publish mode -> broker -> relay; prints the CPRS address
```

A healthy run ends with `path looks good` and the line to type into CPRS:

```
✓ docker           vehu is running (image worldvista/vehu)
⚠ broker publish   published on 127.0.0.1:9430 — bound to loopback, so a VM cannot reach it directly
✓ broker listener  [XWB] handshake on 127.0.0.1:9430 replied 9 bytes (listener live)
✓ relay            relay on 0.0.0.0:19431 forwards to the broker
CPRS should connect to:  10.0.2.2:19431   (s=10.0.2.2 p=19431)
```

If the **relay** line is ✗, start it — either let `doctor` do it, or run it directly:

```bash
v-rpc-debug doctor --fix               # start the relay if needed, then re-check
# or:
v-rpc-debug relay --install            # persistent user service (starts on boot)
v-rpc-debug relay                      # or just run it in the foreground (Ctrl-C to stop)
v-rpc-debug relay --status             # is it listening / installed / active?
v-rpc-debug relay --uninstall          # remove the persistent service
```

`v rpc-debug relay` is a built-in TCP forwarder (no `socat` needed): it discovers
the broker port from `docker inspect` and republishes it on `0.0.0.0:19431` so the
VM can reach it. `--install` writes a `systemd --user` unit (`loginctl
enable-linger` to run it without a login session). It never touches vehu or VistA —
pure host-side port forwarding.

**In CPRS**, set the server/port to what `doctor` printed (vehu via VirtualBox NAT =
`s=10.0.2.2 p=19431`). The cleanest launch is a Windows shortcut with the args
**outside** the path quotes: `Target: "…\CPRSChart.exe" s=10.0.2.2 p=19431`.

> **Prefer to skip the relay entirely?** `doctor` will also tell you: re-run the
> container publishing the broker on all interfaces (`-p 0.0.0.0:9430:9430`) and the
> VM can reach `10.0.2.2:9430` directly — no relay. The relay is the non-invasive
> option when you can't change how the container is started.
>
> For **foia (IRIS-VistA)** the broker port is `19430` (not `9430`); pass it with
> `--broker-port 19430`.

---

## 11. Comparing against the VSL tap

The whole point of the LDJSON format is that its field names line up with the s3tap
envelope (`rpc`, `ts`, `job`, `seq`), so you can join the two captures **offline**
to validate the VSL tap:

1. Capture the oracle: `v rpc-debug capture … --out xwbdebug.ldjson` while you run a
   known workload (e.g. a CPRS login).
2. Capture the same workload through the VSL tap (separate pipeline) → its LDJSON.
3. Compare offline — e.g. load both into DuckDB and diff the set of `rpc` names per
   `job`:

   ```sql
   -- RPCs the broker saw (oracle) but the VSL tap missed
   SELECT rpc FROM 'xwbdebug.ldjson'
   EXCEPT
   SELECT rpc FROM 'vsltap.ldjson';
   ```

The correlation/analysis itself lives in the analysis pipeline, **not** in this tool
— `v rpc-debug` only produces the comparable oracle output.

---

## 12. Limitations

- **Names only.** No RPC results, no request↔response correlation, no payloads
  (those need the VSL hook).
- **Can drop traffic.** `XWBDEBUG` writes to `^XTMP("XWBLOG"_$J)`, one node-tree per
  broker job, and **wipes it at the start of each new connection**. Since process
  IDs recycle, a connection that begins *and* ends entirely between two polls can be
  overwritten before the tool reads it. Use a short `--interval` for busy systems;
  for lossless capture use the VSL tap. (A single, persistent CPRS session is one
  connection, so its full sequence is captured reliably.)
- **Self-purging.** Each log auto-expires ~7 days after its connection (Kernel's
  `^XTMP` cleanup) — fine for live use, not an archive.
- **Second-resolution timestamps** (`$HOROLOG`).

---

## 13. Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `NO_DRIVER` error | The `m-<engine>` driver isn't found — build it (`make build` in `m-ydb`/`m-iris`) or set `M_<ENGINE>_BIN`. |
| `ENGINE` error from `status` | Engine down or unreachable over the driver — check the container is up and the transport/container flags are right. |
| Nothing appears in `tail` | No traffic is hitting the broker, or your `--filter` excludes it; try `--all`, and confirm with `status` that the level is ≥ 2. |
| Level didn't restore (still `ON`) | Either a hard-kill (`kill -9`) skipped the restore, or an **overlapping run**: `tail`/`capture` restore to the level they *found* at start, so if one started while another already armed level 2, it leaves it at 2. Fix: `v rpc-debug disarm`, or run with `--restore-to 1` to force stock on exit. |
| Buffered lines won't go away | They auto-purge in ~7 days; to wipe now use `v rpc-debug clear` (or start a `tail`/`capture` without `--no-clear`). |
| Capture file empty | The workload ran outside the capture window, or every connection was wiped between polls — lower `--interval`, or arm first and capture with `--no-clear`. |
| CPRS can't reach vehu (`WSAECONNREFUSED`) | Run `v rpc-debug doctor` — it pinpoints the broken hop. Usually the broker is published loopback-only and the relay is down: `v rpc-debug doctor --fix` (or `v rpc-debug relay --install`). See [Connecting CPRS to vehu](#10-connecting-cprs-to-vehu-networking). |

---

## 14. References

- **vehu credentials** — [Docker Hub: `worldvista/vehu`](https://hub.docker.com/r/worldvista/vehu)
  (per-user access/verify table); [WorldVistA/docker-vista README](https://github.com/WorldVistA/docker-vista/blob/master/README.md);
  [Hardhats — VEHU/CPRS Demo access/verify codes](https://groups.google.com/g/Hardhats/c/ABYd5QyXmTk).
  Codes were **confirmed against this engine** read-only (`$$EN^XUSHSH` access-code
  hash → `#200` "A" index).
- **CPRS sign-in** — [Signing In to CPRS (WorldVistA)](https://code.worldvista.org/files/clients/OSEHRA_VistA/CPRS/1_0_31_118/Help/Signing_In_to_CPRS.htm).
- **Patient-data figures** — live read-only probe of vehu via the driver seam:
  orders `^OR(100,"AC",DFN)` (#100), notes `^TIU(8925,"C",DFN)` (#8925), visits
  `^AUPNVSIT("C",DFN)` (#9000010), names `^DPT(DFN,0)` (#2). 2026-06-26.
- **Internal** — `docs/archive/v-rpc-implementation-plan.md` (design + tracker, archived);
  `docs/memory/v-rpc-domain.md` (locked design, follow-ups); the shared
  `cprs-rpc-xwbdebug-host-probe` memory and `cprs-rpc-xwbdebug-smoke-test` proposal
  (XWBDEBUG mechanics on the engine side).
