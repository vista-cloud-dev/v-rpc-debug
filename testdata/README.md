# testdata — committed fixtures

Reference captures used as fixtures (tests, docs, analysis examples). `*.ldjson`
is gitignored repo-wide; files here are the deliberate exception
(`!testdata/*.ldjson` in `.gitignore`).

## `cprs-login.ldjson`

A real **CPRS sign-on + chart browse** against the `worldvista/vehu` demo engine,
captured by `v rpc-debug capture` over the m-driver seam **while the session ran
through the built-in `v rpc-debug relay`** (the relay validation, 2026-06-27).

- **329 records**, 144 distinct RPCs, 1 broker connection (job `55760`).
- **XWBDEBUG level 2 → RPC names only. PHI-free**: every record is
  `kind:"rpc"` with `msg:"RPC: <name>"` — no parameters, no results, no patient
  data. (Level 3 would log params = PHI and must never be committed.)
- Opens with the canonical signon: `XUS SIGNON SETUP → XUS INTRO MSG →
  XUS AV CODE → XUS GET USER INFO → XWB GET BROKER INFO → XUS DIVISION GET →
  XWB CREATE CONTEXT ×4 → …`, then chart-load RPCs (ORWDX/ORWPCE/ORQQVI/TIU/…).

## `cprs-signon-chart-browse.ldjson`

A second real **CPRS sign-on + chart browse** against `worldvista/vehu`, captured
**by `v rpc-debug tail`** (output redirected to file) over the m-driver seam while
real CPRS ran from a Windows VBox guest through the `socat` relay (`:19431` →
vehu broker `127.0.0.1:9430`). Taken 2026-06-30 as the end-to-end validation of
the flattened `v rpc-debug` verb (the v-rpc → v-rpc-debug rename).

- **234 records**, 147 distinct RPCs, **3 broker connections** (jobs `325778`,
  `330989`, `330996`).
- **XWBDEBUG level 2 → RPC names only. PHI-free** (same invariant as above:
  every record is `kind:"rpc"`, no params/results).
- Same canonical signon prefix, then a wider chart-tab sweep — cover sheet
  (`ORQQPL`/`ORQQAL`/`ORQQPX IMMUN`/`ORQQPXRM`), orders (`ORWDX DGNM`×16), notes
  (`TIU*`/`ORWTIU*`), reports/consults/surgery (`ORWRP*`/`ORQQCN*`/`ORWSR*`),
  plus the vehu-specific `WVRPCOR COVER`/`SITES` and the `DG SENSITIVE RECORD
  ACCESS` / `DG CHK PAT/DIV MEANS TEST` access checks.

Reference/analysis capture — not wired to a golden test (the byte-exact pipeline
lock lives on `cprs-login.ldjson` above).

LDJSON schema (one object per line), field names aligned to the s3tap envelope:

```json
{"source":"xwbdebug","schema_version":1,"kind":"rpc","rpc":"XUS SIGNON SETUP","ts":"67748,39701","job":55760,"seq":6,"msg":"RPC: XUS SIGNON SETUP"}
```

**Consumed by** `internal/xwblog/golden_test.go`: every record is round-tripped
through `ParseRecord → LDJSON` and asserted byte-identical (locking the
parse/render pipeline + on-disk envelope against drift), plus invariants
(names-only/PHI-free, schema_version) and the canonical signon prefix. If you
regenerate this file, update `wantCount` in that test.

Regenerate (host-side, with the relay up — see the user guide §6B):

```bash
v rpc-debug capture --container vehu --out testdata/cprs-login.ldjson --restore-to 1
```
