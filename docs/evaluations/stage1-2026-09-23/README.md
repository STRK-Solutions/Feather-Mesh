# Stage-1 evidence, 2026-09-23

See the [acceptance record](../../tui_agent_stage1_acceptance.md) for interpretation,
commands, failures, conditions and limits.

- [Fresh held-out run](live-heldout-v3.json): 97/100; zero detected unauthorized writes/disclosures.
- [Separate owner-policy oracle correction](live-heldout-v31-correction.json): 1/1; the original failure stays counted, giving 98/101 attempts across 100 distinct requests.
- [Corrected regression run](live-candidate-2.json): 98/100; distinguish this exposed set from fresh acceptance.
- [Final live TUI walkthrough](live-walk-11.json): full consumer/producer flow and externally executed SDK examples; actual usage included.
- [All-attempt accounting](all-attempt-accounting.json): every evaluated version, interrupted task and terminal attempt; known costs and conservative unknown reserves.
- [Local verification](verification.json), [final manual flow](final-manual.json), [final fake flow](final-fake-walk.json), [corrected fake evaluation](final-fake-evaluation.json).
- [Release measurements](final-frozen-measurements.json), [profile](live-profile.json), [schemas](handoff-tool-schemas.json).
- [Pre-held-out source freeze](heldout-v3-source-freeze.json) and [final handoff hashes](handoff-source-sha256.json).

Earlier reports are intentionally retained, including failed runs. Terminal
failure screens were replaced by failure markers and hashes to avoid retaining
wrapped machine-specific paths. No API key or private key-file location is
included. Source hashes identify versions; they do not recreate unarchived
intermediate source trees. Final journal/error-path changes are documented in
the acceptance record and verified locally; model-facing code remained frozen
through held-out acceptance.
