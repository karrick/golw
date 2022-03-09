package golw

import (
	"bytes"
	"strings"
	"testing"
)

func ensureBuffer(tb testing.TB, got, want []byte) {
	tb.Helper()
	if !bytes.Equal(got, want) {
		tb.Errorf("GOT: %q; WANT: %q", got, want)
	}
}

func ensureError(tb testing.TB, err error, contains ...string) {
	tb.Helper()
	if len(contains) == 0 || (len(contains) == 1 && contains[0] == "") {
		if err != nil {
			tb.Fatalf("GOT: %v; WANT: %v", err, contains)
		}
	} else if err == nil {
		tb.Errorf("GOT: %v; WANT: %v", err, contains)
	} else {
		for _, stub := range contains {
			if stub != "" && !strings.Contains(err.Error(), stub) {
				tb.Errorf("GOT: %v; WANT: %q", err, stub)
			}
		}
	}
}

func ensureOutput(tb testing.TB, got interface{ Bytes() []byte }, want string) {
	tb.Helper()
	if g, w := string(got.Bytes()), want; g != w {
		tb.Errorf("GOT: %q; WANT: %q", g, w)
	}
}
