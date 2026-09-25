# U.06 bounded hosted smoke proposal

Prepared and **explicitly approved by the owner on 2026-09-25; not yet allocated.** Run only after the
private service checks pass and the real staging browser path is available.

| Setting | Exact proposed limit |
| --- | --- |
| Project ceiling | Existing US$100 ledger; no reset or new ledger |
| Live run ceiling | US$1, including retained unknown reservations |
| Single provider request ceiling | US$0.03 |
| Per-user daily ceiling | US$1 within the same finite run |
| Validity | 2026-09-25 08:00 UTC through 2026-09-27 08:00 UTC |
| Model / provider | `deepseek/deepseek-v4.1-flash` / `deepinfra/fp8` |
| Profile | Existing `phase1-demo`; temperature 0, reasoning disabled, 1,024 output tokens |
| Provider price ceilings | US$0.14 input / US$0.42 output per million tokens |
| Reservation overhead | 10% conservative allowance; no credit purchase |
| Retries / fallback | Both disabled |
| Content policy | Approved public metadata; no automatic raw-data disclosure; provider data collection denied |
| Traffic | Five browser tasks, including their bounded tool exchanges; stop on unexpected routing, authority, capture or accounting failure |

The current [endpoint metadata](https://openrouter.ai/api/v1/models/deepseek/deepseek-v4.1-flash/endpoints)
lists the pinned provider and those token prices. OpenRouter lists platform
fees of 5.5% for Standard and 8% for Business; the 10% reservation overhead is
conservative against those rates and is not a new purchase.
[Pricing](https://openrouter.ai/pricing/) and [billing description](https://openrouter.ai/support/).
Fail closed if the key/account has different charges or insufficient credit.

Use these five interactions in the staging user's workspace:

1. Discover the approved climate products and identify their provider.
2. Resolve `ottawa-daily@v2023` exactly and inspect its reuse metadata; verify
   a known table result through the installed SDK without sending raw rows upstream.
3. Resolve `january-maximum-normal@v1991-2020`, explain its climatology interval,
   and verify the bounded raster read locally.
4. Request a copy of the exact table into private workspace output, inspect
   the local review, confirm once, and verify the result/receipt.
5. Propose a practice-project action and decline its review; verify no mutation.

Record actual route, usage, reservations, request/event correlation and the
local confirmation or denial outcome. This is functional compatibility evidence,
not a benchmark. Preserve failed requests and unknown costs. Stop after five
tasks even if budget remains; fixes requiring another live attempt need review
within the approved scope.

The unsigned private allocation is retained at
`.local/demo-deployment/functional-proposal/live-allocation-proposal.json`,
SHA-256 `6e3a93d6fc80ef5f44683bd4634dd539544e0d0e0de6bc153f8fc08682c3e6f2`.
The key file passed owner-only regular-file checks without exposing its contents.
An authenticated, read-only [key-status request](https://openrouter.ai/docs/api/api-reference/api-keys/get-current-api-key)
also passed; its remaining key allowance covers this US$1 run. Historical key
usage is retained privately as the pre-smoke baseline. A key allowance does not
prove the account's wallet balance; no credit purchase or inference was made.
The project ledger was read at revision 1: zero runs, zero settled project cost,
US$100 allowance. The owner approved this exact finite smoke and a second
regular staging identity in the takeover conversation. Issuing the signed
allocation and any inference remain pending the private service checks.
