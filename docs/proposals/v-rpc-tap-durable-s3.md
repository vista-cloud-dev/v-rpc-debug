---
title: v-rpc-tap — durable RPC-broker tap to S3 (independent KIDS package + host streamer)
status: draft
version: v0.1.0
created: 2026-06-27
last_modified: 2026-06-27
doc_type: [PROPOSAL]
layer: v
---

# v-rpc-tap — durable RPC-broker tap → S3

A safety-critical redesign of the durable RPC tap, informed by what we now have
that we didn't when it was first built: the **proven `v rpc debug` XWBDEBUG
oracle** (zero-install activate/deactivate of a *native* feature) and the
**`v pkg` KIDS lifecycle** as the only sanctioned install path. This proposal
answers: *where does the durable tap fit, and how do we make a function that can
break every CPRS user's traffic acceptably safe?*

## What already exists (don't rebuild — re-home + harden)

The M tap subsystem is **built and live-proven**, sitting in `v-stdlib/src/`:

| Routine | Role | Status |
|---|---|---|
| `VSLTAP` | non-interference capture core; bounded `^XTMP("VSLTAP")` ring, FU-4 naked-ref fence, FU-8 atomic seq, FU-9 always-on-ring/egress split | CURRENT, proven |
| `VSLRPCTAP` | in-line RPC tee at the chokepoint (`capture^VSLRPCTAP(.msg)`) | CURRENT |
| `VSLRPCWRAP` | the broker-dispatch splice glue (fenced side-calls) | CURRENT — **pins the wrong seam, see below** |
| `VSLS3` | S3 egress (LDJSON envelope, SigV4) | CURRENT |
| `VSLHL7TAP`/`VSLTAPHL`/`VSLTAPFC` | HL7 store-tailer / latency watchdog / fidelity comparator | CURRENT |

Non-interference was **proven end-to-end on live foia/IRIS (2026-06-23)** (scalar
+ global-array paths identical armed vs off; naked-ref + `$T` preserved; back-out
byte-identical). The schema-v1 LDJSON envelope is **frozen**
(`docs/design/s3tap-envelope-schema-lock.md`).

**What was deleted** (2026-06-25, "[[never-use-bespoke-installer]]"): only the
*installer* machinery — `v pkg wrap-rpc`, `internal/wrapsplice`, and the M
install/back-out routines `VSLTAPBO`/`VSLBLD`/`VSLENV`. **Do not revive them.**
Install/back-out is now strictly `v pkg install`/`uninstall` of a KIDS build.

### Correction — the dispatch seam (decisive)

The first build pinned the splice to **`CALLP^XWBBRK`** — the **OLD/`{XWB}`
callback** broker. Corpus + routine source confirm **modern CPRS uses the NEW
broker**, dispatching at **`CAPI^XWBPRS`** (`D @XWBCALL`, `XWBPRS.m:212`), where
RPC name (`XWB(2,"RPC")`), params (`XWB("PARAM")`), and result (`XWBY`) are all in
scope simultaneously, *before* the socket is re-`USE`d. Our own `v rpc debug`
capture logged via `XWBPRS.m:64` — i.e. vehu CPRS is the NEW broker. **The tap
must re-pin its one-line splice to `CAPI^XWBPRS`** (and may patch `XWBBRK2`'s
`CAPI` too, to cover any site still on the callback path). There is **no supported
around-RPC hook** in XWB — patching exactly one dispatch line is unavoidable and
is the least-invasive option that captures correlated name+params+result.

## Recommendation — where it fits

**Split the function at the waterline by risk, not into one monolith.** Two
artifacts with very different risk profiles:

### 1. Engine-resident tap = its own **standalone KIDS package** (the risky half)

- A tap-only KIDS build (e.g. `VSL TAP 1.0`) containing the `VSL*TAP`/`VSLRPCWRAP`
  routines **+ the one-line `CAPI^XWBPRS` dispatch patch**, packaged **separately
  from `vsl.build.json`** so it never rides with the VSL library and is
  **independently installable/uninstallable**. This is exactly the
  `cprs-rpc-broker-hook-kids` decision and satisfies the "independent of any other
  module, separate from v-stdlib" requirement.
- **Source-home it in v-stdlib for now** (a second `kids/vsltap.build.json`) —
  don't move live-proven, dual-engine-tested routines as step one. Graduation to a
  dedicated `v-tap` M repo is a later option if hard repo-isolation is wanted; the
  *install* is already independent regardless of source repo.
- Installed/removed **only** via `v pkg install vsltap.build.json` /
  `v pkg uninstall` — with `--auto-snapshot` so the patched `XWBPRS` pre-image is
  captured and back-out is `--verify` byte-clean (class-1 pure-overwrite, the
  strongest guarantee v-pkg gives).

### 2. Host-side control + streamer + validator = a **new `v-rpc-tap` module**, mounted as `v rpc tap`

- A Go module (depends on **v-rpc** for the LDJSON envelope + the XWBDEBUG oracle,
  **pkgcli** to install the KIDS build, **m-driver-sdk** for the seam) that:
  arms/disarms the tap, **drains the ring → S3 from the host** (so the engine never
  does network I/O in the RPC path — see Safety), watches health, and **validates
  captured traffic against `v rpc debug capture`**.
- Mounted under the umbrella as **`v rpc tap`** for UX cohesion (sibling of
  `v rpc debug`/`doctor`), but kept in its **own repo** so the risky orchestration
  never destabilizes v-rpc's pristine zero-install diagnostic surface.

**Why not just extend v-rpc?** v-rpc's whole safety story is "zero-install, just
toggles a native feature." Bolting an engine-package installer into it muddies
that. A separate `v-rpc-tap` that *reuses* v-rpc keeps the safe oracle and the
risky tap clearly separated while still presenting as `v rpc tap`.

**Why not a single monolithic `v-rpc-tap` holding the M code too?** It would
duplicate the envelope/oracle/doctor that already live in v-rpc and split the
validation story. Keep the M source where it's proven (v-stdlib), ship it as an
independent build, and put host control in the new module.

> **This is the key decision for owner sign-off** (see Open questions): (A) host
> control in a new `v-rpc-tap` repo [recommended], vs (B) an explicitly-marked
> advanced group inside v-rpc; and (C) M source stays in v-stdlib as a 2nd build
> [recommended] vs (D) graduates to a dedicated `v-tap` M repo.

## Safety architecture — least-invasive, auto-failover (the heart)

The tap is **observe-only** and must **fail open**: any tap problem leaves the RPC
completely untouched, and the tap **turns itself off**. Layered defenses, cheapest
first:

1. **Inline per-RPC kill flag (instant dead-man).** The splice's *first* action is
   `Q:'$G(^XTMP("VSLTAP","ON"))` — one `$GET` per RPC, negligible cost, **fails
   safe** (undefined → OFF). Unlike `XWBDEBUG` (read once per connection, so it
   can't stop live sessions), this is checked **every dispatch** and is killable
   **instantly, system-wide**, by clearing one node. This is the master switch.

2. **Fail-open error fence.** The entire side-call runs under its own
   `$ETRAP`/`$ZTRAP` that, on *any* error: clears the kill flag (auto-disable),
   resets `$ECODE`, and returns so the RPC proceeds. The tap must **never** let an
   error reach the broker's `ETRAP^XWBTCPM`, which does `X "HALT "` and kills the
   CPRS session. **First error → tap self-disables.**

3. **Non-interference fences** (already built, re-verify on `XWBPRS`): FU-4
   naked-reference save/restore (`$ZREFERENCE`); **no I/O on `XWBTDEV`**, never
   touch `$IO`; **NEW every tap variable** (broker saved-var list `VARLST^XWBLIB`);
   **no `LOCK`** (never argumentless `L `); don't mutate `$TEST`/`$ECODE`.

4. **Bounded, non-blocking, fire-and-forget capture.** Write to the bounded
   `^XTMP("VSLTAP")` ring (drop-oldest on overflow, never block); **the engine does
   zero network I/O** — egress is a separate host drain. This removes S3 latency,
   DNS, and TLS from the RPC path entirely.

5. **Lease / host dead-man.** The host streamer refreshes a lease
   (`^XTMP("VSLTAP","LEASE")=$H+ttl`) every N seconds; the splice gates on
   `Q:$$expired(lease)`. So if the **host streamer dies, S3 is unreachable, or the
   ring fills**, the engine tap **auto-disables within one TTL** — the tap is OFF
   unless something healthy is actively maintaining it.

6. **Latency watchdog (auto-disable on interference).** `VSLTAPHL` (built) measures
   per-RPC armed-vs-baseline overhead; breach of a threshold → clear the kill flag.
   Directly satisfies "turn itself off if any problem with CPRS traffic flow."

7. **Staged rollout + canary.** XWBDEBUG-only smoke (done) → bare engine →
   single-user vehu → time-boxed `--duration` arm with the latency watchdog →
   widen. Never a blind always-on flip.

## Install / uninstall via v-pkg (and the one gap)

- **Independent unit:** own `vsltap.build.json` (own `package`/version,
  `allowLongNames`), own `.KID`, own `#9.7` install — zero coupling to v-stdlib's
  `VSL` install; install/uninstall in any order.
- **Patch reversibility:** the only national-code change is the one `XWBPRS` line;
  install with `--auto-snapshot`, uninstall restores the pre-image and
  `--verify` proves byte-clean — the exact machinery that proved byte-clean back-out
  before.
- **The gap:** v-pkg's `build.json` `envCheck` and the install-time PRE/POST hooks
  are **not wired into the live install path** (validated-but-dead today). So **do
  not rely on an install-time precondition gate.** Instead, the tap ships
  **OFF by default** (kill flag unset, no lease) — installing the routines changes
  *nothing* until an explicit `v rpc tap arm`. This is safer than an install hook
  anyway: arming is a separate, observable, instantly-reversible step. (If we later
  want an install-time engine precheck, the clean fix is to extend the v-pkg
  builder to emit a `#9.6` environment-check — a separate v-pkg change, not a
  bespoke installer.)

## Validation against the `v rpc debug` oracle (the user's requirement)

The durable tap and the XWBDEBUG oracle now share an **identical LDJSON field
contract** (`rpc`/`ts`/`job`/`seq`) — that alignment was built into v-rpc
deliberately. So validation is a straight offline join over the same workload:

- Capture a known workload (e.g. the committed `testdata/cprs-login.ldjson`
  fixture, 329 RPCs) **both ways**: `v rpc debug capture` (oracle) and the durable
  tap.
- `v rpc tap validate` diffs the two: the oracle's RPC-name set per job must be a
  **subset** of the tap's (the tap is complete + adds params/results the oracle
  lacks). Any oracle RPC the tap missed is a fidelity bug.
- This makes the **already-proven, zero-risk oracle the acceptance gate** for the
  risky tap — the strongest possible validation, and independent (different
  mechanism entirely).

## Waterline & governance

- Engine tap routines + the `v rpc tap` control are **layer `v`** (Broker/KIDS).
  Host→engine only via `mdriver.Client`/`v pkg` (rule 3). `v rpc-tap → v-rpc → m`
  one-way (rule 1).
- `v rpc tap` is a legitimate **actuator** domain (installs + arms live-engine
  code through the seam) per the v-cli-domain-eligibility ADR.
- Install/back-out strictly `v pkg` — no bespoke installer ([[never-use-bespoke-installer]]).

## Phased plan (TDD; reuse > rebuild)

- **P0 — re-pin the seam:** confirm on vehu which dispatch path live CPRS hits
  (XWBDEBUG already says NEW/`XWBPRS`); re-pin `VSLRPCWRAP` splice to `CAPI^XWBPRS`;
  re-prove non-interference (the existing 3-arm bench) on `XWBPRS`.
- **P1 — standalone KIDS build:** author `kids/vsltap.build.json` (tap-only);
  `v pkg build` → `.KID`; live `install --auto-snapshot` → `uninstall --verify`
  byte-clean on vehu (YDB) + foia-t12 (IRIS).
- **P2 — safety harness:** inline kill flag + fail-open fence + lease/dead-man +
  latency-watchdog auto-disable; OFF-by-default; tests for each failure mode
  (forced tap error must not perturb the RPC; lease lapse disables within TTL).
- **P3 — `v-rpc-tap` host module:** `v rpc tap {arm,disarm,status,stream,validate}`;
  host ring-drain → S3 (reuse Go S3 libs host-side, or keep `VSLS3` engine-side as
  a fallback — decide in P3); `validate` against the oracle fixture.
- **P4 — staged live rollout** with the canary + watchdog; widen only on green.

## Open questions (owner)

1. **Host home:** new `v-rpc-tap` repo [recommended] vs an advanced group inside
   v-rpc.
2. **M source home:** stay in v-stdlib as a 2nd standalone build [recommended] vs
   graduate to a dedicated `v-tap` M repo.
3. **Egress side:** host-side ring-drain → S3 [recommended, least invasive] vs the
   existing engine-side `VSLS3` PUT.
4. **Seam coverage:** patch only `CAPI^XWBPRS` (modern CPRS) or also `XWBBRK2` (old
   callback path) for site generality?
5. **Namespace:** keep the `VSLTAP`/`VSLRPC` sub-prefixes under a separate package,
   or mint a dedicated tap namespace for clean independence?

## References

- Reuse (CURRENT): `v-stdlib/src/VSL*TAP*.m`, `VSLRPCWRAP.m`, `VSLS3.m`;
  `docs/design/s3tap-envelope-schema-lock.md`;
  `docs/proposals/implemented/rpc-traffic-s3-streaming*.md` (capture design +
  non-interference proof stand; install mechanism superseded).
- Sanctioned path: `docs/proposals/considering/cprs-rpc-broker-hook-kids.md`
  (standalone build); [[never-use-bespoke-installer]];
  `docs/background/m-v-waterline-adr.md`.
- Dispatch internals: XWB `CAPI^XWBPRS` (`XWBPRS.m:212`), `XWBTCPM.m` (listener +
  `ETRAP` `HALT`), `XWBDLOG.m` (`^XTMP("XWBLOG")`). Oracle: this repo's
  `internal/xwblog` + `v rpc debug`, and `testdata/cprs-login.ldjson`.
