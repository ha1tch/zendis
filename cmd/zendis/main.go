// Command zendis disassembles Z80/Z80N machine code from a binary
// file. This is a thin CLI wrapper: flag parsing and process exit
// codes live here, but the actual decode-loop orchestration and every
// output format are pkg/render's -- the same relationship
// pkg/disasm's Decode/DecodeTarget already has with this command.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zendis/pkg/render"
)

func main() {
	org := flag.Uint("org", 0, "load address of the first byte (decimal, or 0x-prefixed hex)")
	targetName := flag.String("target", "z80", "CPU target: z80 (Z80N-only opcodes stay undefined) or z80n")
	warnZ80N := flag.Bool("warn-z80n", true, "under --target=z80, warn on stderr when an undefined opcode matches a Z80N instruction")
	labels := flag.Bool("labels", true, "replace JP/JR/CALL/DJNZ target addresses with generated labels (Lxxxx) where no hints-file name overrides them")
	hintsPath := flag.String("hints", "", "path to a hints file (label/comment/data/entry directives -- see README)")
	format := flag.String("format", "listing", "output format: listing (default), plain/zenas, pasmo, sjasmplus, snasm (all four re-assemblable), or json")
	symPath := flag.String("sym", "", "write a symbol file (format matches --format: pasmo's/zenas's shape for plain/zenas/pasmo, sjasmplus's shape for sjasmplus) -- requires --format to be one of those four; not available for snasm, see README")
	flag.Parse()

	var target disasm.Target
	switch *targetName {
	case "z80":
		target = disasm.TargetZ80
	case "z80n":
		target = disasm.TargetZ80N
	default:
		fmt.Fprintf(os.Stderr, "zendis: unknown --target %q (want z80 or z80n)\n", *targetName)
		os.Exit(2)
	}
	switch *format {
	case "listing", "plain", "zenas", "pasmo", "sjasmplus", "snasm", "json":
	default:
		fmt.Fprintf(os.Stderr, "zendis: unknown --format %q (want listing, plain, zenas, pasmo, sjasmplus, snasm, or json)\n", *format)
		os.Exit(2)
	}
	if *symPath != "" {
		switch *format {
		case "plain", "zenas", "pasmo", "sjasmplus":
		case "snasm":
			fmt.Fprintln(os.Stderr, "zendis: --sym is not available for --format=snasm -- SNASM's own -sym/-vice flags produce no output in the tested release (V3.2.3.0), so there's no working format to match; its -map output is a different kind of artifact (source-line debug info, not just a symbol table) and isn't implemented here")
			os.Exit(2)
		default:
			fmt.Fprintf(os.Stderr, "zendis: --sym requires --format to be plain, zenas, pasmo, or sjasmplus (got %q)\n", *format)
			os.Exit(2)
		}
	}

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: zendis [--org ADDR] [--hints FILE] [--format listing|plain|zenas|pasmo|sjasmplus|snasm|json] [--sym FILE] <file.bin>")
		os.Exit(2)
	}

	code, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
		os.Exit(1)
	}

	hints := &disasm.Hints{}
	if *hintsPath != "" {
		f, err := os.Open(*hintsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
			os.Exit(1)
		}
		h, err := disasm.ParseHints(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
			os.Exit(1)
		}
		hints = h
	}

	d := render.NewDisassembly(code, uint16(*org), target, hints, *labels)
	exitCode := 0
	if d.TruncatedTail() {
		exitCode = 1
	}

	switch *format {
	case "listing":
		d.WriteListing(os.Stdout, *warnZ80N, os.Stderr)
	case "plain", "zenas", "pasmo", "sjasmplus", "snasm":
		d.WriteAssemblySource(os.Stdout, render.Dialect(*format), os.Stderr)
	case "json":
		if err := d.WriteJSON(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
			os.Exit(1)
		}
	}

	if *symPath != "" {
		symbols := d.Symbols()
		f, err := os.Create(*symPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", err)
			os.Exit(1)
		}
		var werr error
		if *format == "sjasmplus" {
			werr = render.WriteSjasmplusSym(f, symbols)
		} else {
			werr = render.WritePasmoSym(f, symbols)
		}
		cerr := f.Close()
		if werr != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", werr)
			os.Exit(1)
		}
		if cerr != nil {
			fmt.Fprintf(os.Stderr, "zendis: %v\n", cerr)
			os.Exit(1)
		}
	}

	os.Exit(exitCode)
}
