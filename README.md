# zendis

A disassembler for the Zilog Z80 and the Z80N (the Spectrum Next's
extended Z80), written in Go. A sibling of
[zen80](https://github.com/ha1tch/zen80) (the CPU core) and
[zenas](https://github.com/ha1tch/zenas) (the assembler): what zendis
writes, zenas assembles back to the same bytes.

```
go build -o zendis ./cmd/zendis
./zendis --org=0x8000 game.bin                    # a listing
./zendis --tape=game.tap --format=zenas           # source that assembles again
./zendis --nex=game.nex --blocks                  # what a Next executable holds
./zendis --nex=game.nex --symbols=game.map        # with the program's own names
```

```
8000:  00          NOP
8001:  3E 2A       LD A,0x2A
8003:  DD 21 00 80 LD IX,0x8000
8007:  DD 7E 05    LD A,(IX+0x05)
800A:  CB 7C       BIT 7,H
800C:  ED B0       LDIR
800E:  C3 00 00    JP L0000
```

## What it does

| | |
|---|---|
| **Instructions** | Every Z80 form, the undocumented ones included; the Z80N's 29 with `--target=z80n` (the default for a NEX file). Branch targets become labels. |
| **Input** | A raw binary; a tape (`.tap`, `.tzx`, `.pzx`); a snapshot (`.sna`, `.z80`, `.szx`, 48K and 128K); an `.rzx` recording; a Spectrum Next executable (`.nex`). `--blocks` lists the parts of a tape or a NEX file, and `--block`, `--bank`, `--page`, `--org` and `--length` choose one. |
| **Code and data** | Everything is decoded as instructions unless it is said to be data: `--data=ADDR:LENGTH[:KIND]`, a hints file, or `--as=KIND` for a whole block. A kind is one of zenas's data directives: `bytes`, `hex`, `words`, `text`, `fill`. |
| **Names** | A hints file (labels, comments, data ranges) and other tools' symbol files: a CSpect map, a zenas or pasmo `.sym`, an sjasmplus `--sym` file, a z88dk `.map`. `--sym` and `--map` write them. |
| **Output** | `--format=listing`, source for `zenas`, `pasmo`, `sjasmplus` or `snasm`, or `json`. |
| **Library** | `pkg/api` is the command as one call; `pkg/disasm`, `pkg/load` and `pkg/render` are its parts. |

It decodes straight through from the start address: there is no tracing
of the program's flow, so data that is not marked comes out as
instructions. [`docs/TRACKING.md`](docs/TRACKING.md) has that and the
rest of what is open.

## Documents

| Document | What is in it |
|---|---|
| [`docs/USAGE.md`](docs/USAGE.md) | The command: the listing, each kind of input, NEX files, code and data, hints, symbol files, the output formats |
| [`docs/LIBRARY.md`](docs/LIBRARY.md) | The packages, with the library call for each option, and zenas and zendis in one process |
| [`docs/TRACKING.md`](docs/TRACKING.md) | What is open |
| [`docs/KNOWN_ISSUES.md`](docs/KNOWN_ISSUES.md) | Limits that are meant, and the checks that do not run with `go test` |
| [`docs/RESOLVED.md`](docs/RESOLVED.md), [`CHANGELOG.md`](CHANGELOG.md) | What was closed, and what shipped |

## Requirements

- Go 1.25 or later

## Contact

Email: h@ual.li

https://oldbytes.space/@haitchfive

## Licence

Copyright (c) 2026 haitch

Licensed under the Apache License, Version 2.0: https://www.apache.org/licenses/LICENSE-2.0
