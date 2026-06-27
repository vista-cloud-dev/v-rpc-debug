# testdata — committed fixtures

Reference captures used as fixtures (tests, docs, analysis examples). `*.ldjson`
is gitignored repo-wide; files here are the deliberate exception
(`!testdata/*.ldjson` in `.gitignore`).

## `cprs-login.ldjson`

A real **CPRS sign-on + chart browse** against the `worldvista/vehu` demo engine,
captured by `v rpc debug capture` over the m-driver seam **while the session ran
through the built-in `v rpc relay`** (the relay validation, 2026-06-27).

- **329 records**, 144 distinct RPCs, 1 broker connection (job `55760`).
- **XWBDEBUG level 2 → RPC names only. PHI-free**: every record is
  `kind:"rpc"` with `msg:"RPC: <name>"` — no parameters, no results, no patient
  data. (Level 3 would log params = PHI and must never be committed.)
- Opens with the canonical signon: `XUS SIGNON SETUP → XUS INTRO MSG →
  XUS AV CODE → XUS GET USER INFO → XWB GET BROKER INFO → XUS DIVISION GET →
  XWB CREATE CONTEXT ×4 → …`, then chart-load RPCs (ORWDX/ORWPCE/ORQQVI/TIU/…).

LDJSON schema (one object per line), field names aligned to the s3tap envelope:

```json
{"source":"xwbdebug","schema_version":1,"kind":"rpc","rpc":"XUS SIGNON SETUP","ts":"67748,39701","job":55760,"seq":6,"msg":"RPC: XUS SIGNON SETUP"}
```

Regenerate (host-side, with the relay up — see the user guide §6B):

```bash
v-rpc debug capture --container vehu --out testdata/cprs-login.ldjson --restore-to 1
```
