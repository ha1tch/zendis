# zendis — Wave Tracking

Updated: 2026-08-28

## 1. Progress at a glance

```
Wave 1  baseline features           ██████████████████░░   ~88%  (3.5/4 items)
Wave 2  traversal -- code/data classification and control-flow discovery  █████░░░░░░░░░░░░░░░   ~25%  (0.5/2 items)
Wave 3  cli input/output flexibility -- symbol annotation and alternate output  ███████████████░░░░░   ~75%  (1.5/2 items)
Wave 4  decoder verification rigor -- fuzz and property testing  ░░░░░░░░░░░░░░░░░░░░     0%  (0/1 items)
```

Overall by item count: 5.5 of 9 items ≈ **61%**

### Wave 1 — baseline features (4 items, ideal 4.0d, added 2026-08-29)

| # | Summary | Status | Register item |
|---|---|---|---|
| 1 | Resolve Z80N ED-prefixed extended opcodes to their mnemonics | ✓ | T-1 |
| 2 | Add label/symbol resolution to the CLI listing | ✓ | T-2 |
| 3 | Build a round-trip verification harness against zenas for the DD/FD table | ◐ | T-3 |
| 4 | Cross-verify DDCB/FDCB undocumented copy-to-register forms against a second source | ✓ | T-4 |

**Wave 1: 3/4, T-3 still open (pending a zen80 release for two oracle checks).**

### Wave 2 — traversal -- code/data classification and control-flow discovery (2 items, ideal 3.0d, added 2026-08-29)

| # | Summary | Status | Register item |
|---|---|---|---|
| 5 | Mark known non-code byte ranges so linear sweep stops decoding data as instructions | ◐ | T-05 |
| 6 | Traverse from one or more entry points following JP/CALL/JR/DJNZ targets instead of a flat linear sweep | ☐ | T-06 |

**Wave 2: 0/2, not started.**

### Wave 3 — cli input/output flexibility -- symbol annotation and alternate output (2 items, ideal 2.5d, added 2026-08-29)

| # | Summary | Status | Register item |
|---|---|---|---|
| 7 | Accept a symbol file / annotation input so user-supplied names and comments survive re-runs | ◐ | T-07 |
| 8 | Support alternate output forms beyond the single fixed listing format | ✓ | T-08 |

**Wave 3: 1/2, T-07 still open (symbol-file input/annotation, not yet consumed for its own sake beyond what T-05/T-08 already use it for).**

### Wave 4 — decoder verification rigor -- fuzz and property testing (1 item, ideal 2.0d, added 2026-08-29)

| # | Summary | Status | Register item |
|---|---|---|---|
| 9 | Add fuzzing/property-based tests asserting decode invariants (length consumed, no panics, defined-or-reported) across the opcode space | ☐ | T-09 |

**Wave 4: 0/1, not started.**

---

