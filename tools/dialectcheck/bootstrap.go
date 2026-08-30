// Bootstrap: locates real pasmo and sjasmplus binaries for
// dialectcheck's round-trip tests, in order:
//  1. Already on $PATH (respects a system-installed version).
//  2. Already built and cached from a previous run
//     (os.UserCacheDir()/zendis-dialectcheck/<name>).
//  3. Neither: clone the real source and build it, then cache the
//     result for next time.
//
// Unlike tools/roundtrip (zenas is a Go module dependency, always
// available via `go mod tidy`), pasmo and sjasmplus are standalone C++
// programs with no Go bindings -- this step is genuinely
// environment-dependent (needs git, g++, and for sjasmplus, make) in a
// way the zenas-based harness isn't. Building is slow (sjasmplus
// especially); the cache exists specifically so a normal re-run of
// this harness doesn't rebuild from scratch every time.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// dialectCacheDir returns (creating if needed) the directory pasmo and
// sjasmplus binaries are cached in across runs.
func dialectCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("finding user cache dir: %w", err)
	}
	dir := filepath.Join(base, "zendis-dialectcheck")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating cache dir %s: %w", dir, err)
	}
	return dir, nil
}

// requireTool checks a build-time dependency is on PATH, returning a
// clear error naming what's missing rather than letting exec.Command
// fail with a bare "file not found" further down.
func requireTool(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s not found on PATH -- required to build from source", name)
	}
	return nil
}

// findOrBuildPasmo returns a path to a working pasmo binary.
func findOrBuildPasmo() (string, error) {
	if p, err := exec.LookPath("pasmo"); err == nil {
		return p, nil
	}

	cacheDir, err := dialectCacheDir()
	if err != nil {
		return "", err
	}
	cached := filepath.Join(cacheDir, "pasmo")
	if info, err := os.Stat(cached); err == nil && info.Mode()&0111 != 0 {
		return cached, nil
	}

	fmt.Println("pasmo not found on PATH or in cache -- building from source (github.com/VWarlock/Z80_Pasmo_cmake, a faithful build of real pasmo 0.5.5)...")
	if err := requireTool("git"); err != nil {
		return "", err
	}
	if err := requireTool("g++"); err != nil {
		return "", err
	}

	src, err := os.MkdirTemp("", "pasmo-src-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(src)

	if err := runCommand(".", "git", "clone", "--depth", "1",
		"https://github.com/VWarlock/Z80_Pasmo_cmake.git", src); err != nil {
		return "", fmt.Errorf("cloning pasmo source: %w", err)
	}

	sources := []string{
		"asm.cxx", "asmerror.cxx", "asmfile.cxx", "cpc.cxx", "macro.cxx",
		"nullstream.cxx", "pasmo.cxx", "pasmotypes.cxx", "spectrum.cxx",
		"tap.cxx", "token.cxx", "tzx.cxx",
	}
	args := append([]string{"-O2", `-DVERSION="0.5.5"`, "-DNDEBUG", "-std=c++14", "-o", cached}, sources...)
	if err := runCommand(src, "g++", args...); err != nil {
		return "", fmt.Errorf("compiling pasmo: %w", err)
	}
	fmt.Println("pasmo built and cached at", cached)
	return cached, nil
}

// findOrBuildSjasmplus returns a path to a working sjasmplus binary.
// Built with USE_LUA=0: the bundled LuaBridge dependency is a git
// submodule a shallow clone doesn't fetch, and dialectcheck's tests
// don't need Lua scripting.
func findOrBuildSjasmplus() (string, error) {
	if p, err := exec.LookPath("sjasmplus"); err == nil {
		return p, nil
	}

	cacheDir, err := dialectCacheDir()
	if err != nil {
		return "", err
	}
	cached := filepath.Join(cacheDir, "sjasmplus")
	if info, err := os.Stat(cached); err == nil && info.Mode()&0111 != 0 {
		return cached, nil
	}

	fmt.Println("sjasmplus not found on PATH or in cache -- building from source (github.com/z00m128/sjasmplus, USE_LUA=0)...")
	if err := requireTool("git"); err != nil {
		return "", err
	}
	if err := requireTool("make"); err != nil {
		return "", err
	}
	if err := requireTool("g++"); err != nil {
		return "", err
	}

	src, err := os.MkdirTemp("", "sjasmplus-src-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(src)

	if err := runCommand(".", "git", "clone", "--depth", "1",
		"https://github.com/z00m128/sjasmplus.git", src); err != nil {
		return "", fmt.Errorf("cloning sjasmplus source: %w", err)
	}
	if err := runCommand(src, "make", "USE_LUA=0"); err != nil {
		return "", fmt.Errorf("building sjasmplus: %w", err)
	}

	built := filepath.Join(src, "sjasmplus")
	if err := copyFile(built, cached); err != nil {
		return "", fmt.Errorf("caching built sjasmplus binary: %w", err)
	}
	if err := os.Chmod(cached, 0755); err != nil {
		return "", err
	}
	fmt.Println("sjasmplus built and cached at", cached)
	return cached, nil
}

// runCommand runs a command in dir, streaming its output directly to
// this process's stdout/stderr (the build steps are slow enough --
// sjasmplus especially -- that silent hanging would be worse than
// verbose output) and returning an error if it exits non-zero.
func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// findSnasm locates a real SNasm.exe (Mike Dailly's SNASM assembler)
// and confirms mono can run it. Unlike findOrBuildPasmo/
// findOrBuildSjasmplus, this never attempts to fetch or build
// anything: SNASM is freely distributed but not open source (no
// public repository, no license permitting redistribution), so there
// is nothing this function is entitled to download on the user's
// behalf. It only ever looks in two places -- $PATH (for a "snasm"
// wrapper or renamed executable) and this tool's own cache directory
// (for a manually-placed SNasm.exe) -- and if neither has it, says so
// plainly and points at where to get it, rather than guessing or
// silently skipping SNASM's checks.
//
// SNasm.exe is a Mono/.NET assembly (confirmed via `file`), not a
// native binary, so every invocation goes through `mono
// <path> <args>`, not the binary directly.
func findSnasm() (monoPath, snasmPath string, err error) {
	mono, err := exec.LookPath("mono")
	if err != nil {
		return "", "", fmt.Errorf("mono not found on PATH -- required to run SNasm.exe (it's a Mono/.NET assembly, not a native binary)")
	}

	if p, err := exec.LookPath("snasm"); err == nil {
		return mono, p, nil
	}

	cacheDir, err := dialectCacheDir()
	if err != nil {
		return "", "", err
	}
	cached := filepath.Join(cacheDir, "SNasm.exe")
	if _, err := os.Stat(cached); err == nil {
		return mono, cached, nil
	}

	return "", "", fmt.Errorf(
		"SNasm.exe not found on PATH (as \"snasm\") or at %s\n"+
			"SNASM is freely distributed but not open source, so this harness\n"+
			"can't download it the way it builds pasmo and sjasmplus from source.\n"+
			"Get it from https://mdf200.itch.io/snasm and either put a \"snasm\"\n"+
			"wrapper/rename on $PATH, or place SNasm.exe at the path above.",
		cached)
}
