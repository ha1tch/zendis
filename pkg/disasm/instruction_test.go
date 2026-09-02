package disasm

import "testing"

// TestParseTarget_RoundTrip confirms ParseTarget is the genuine
// reverse of Target.String -- every String() output round-trips back
// through ParseTarget to the same Target value.
func TestParseTarget_RoundTrip(t *testing.T) {
	for _, target := range []Target{TargetZ80, TargetZ80N} {
		s := target.String()
		got, err := ParseTarget(s)
		if err != nil {
			t.Errorf("ParseTarget(%q): %v", s, err)
			continue
		}
		if got != target {
			t.Errorf("ParseTarget(%q) = %v, want %v", s, got, target)
		}
	}
}

// TestParseTarget_Unknown confirms an unrecognised string is a clear
// error, not a silent zero-value TargetZ80 -- the same distinction
// cmd/zendis's own former inline switch made via its default case.
func TestParseTarget_Unknown(t *testing.T) {
	if _, err := ParseTarget("bogus"); err == nil {
		t.Error("ParseTarget(\"bogus\") = nil error, want a non-nil error")
	}
	if _, err := ParseTarget(""); err == nil {
		t.Error("ParseTarget(\"\") = nil error, want a non-nil error")
	}
}
