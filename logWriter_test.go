package golw

import (
	"bytes"
	_ "embed"
	"io"
	"testing"
)

//go:embed 2600-0.txt
var novel []byte

func TestNewLogWriter(t *testing.T) {
	t.Run("no buffer", func(t *testing.T) {
		// t.Skip("works")
		// Create the following data pipeline:
		//
		//     novel -> limit reader -> log writer

		cfg := &Config{
			BaseNamePrefix: "no-buffer",
			BufferSizeMax:  -1,
			Directory:      tempdir,
			MaxBytes:       512,
		}

		lw, err := NewLogWriter(cfg)
		ensureError(t, err)

		t.Logf("%#v\n", lw)

		const total = int64(8 * 1024)
		lr := io.LimitReader(bytes.NewReader(novel), total)

		nw, err := io.CopyBuffer(lw, lr, make([]byte, 1024))
		ensureError(t, err)

		if got, want := nw, total; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}

		ensureError(t, lw.Close())
	})

	t.Run("buffer smaller than file", func(t *testing.T) {
		// t.Skip("works")
		// Create the following data pipeline:
		//
		//     novel -> limit reader -> per line reader -> log writer

		cfg := &Config{
			BaseNamePrefix: "buffer-smaller-than-file",
			BufferSizeMax:  512,
			Directory:      tempdir,
			MaxBytes:       1024,
		}

		lw, err := NewLogWriter(cfg)
		ensureError(t, err)

		t.Logf("%#v\n", lw)

		const total = int64(8 * 1024)
		lr := io.LimitReader(bytes.NewReader(novel), total)

		nw, err := io.CopyBuffer(lw, lr, make([]byte, 2048))
		ensureError(t, err)

		if got, want := nw, total; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}

		ensureError(t, lw.Close())
	})

	t.Run("file smaller than buffer", func(t *testing.T) {
		// Create the following data pipeline:
		//
		//     novel -> limit reader -> per line reader -> log writer

		cfg := &Config{
			BaseNamePrefix: "file-smaller-than-buffer",
			BufferSizeMax:  1024,
			Directory:      tempdir,
			MaxBytes:       512,
		}

		lw, err := NewLogWriter(cfg)
		ensureError(t, err)

		t.Logf("%#v\n", lw)

		const total = int64(8 * 1024)
		lr := io.LimitReader(bytes.NewReader(novel), total)

		nw, err := io.CopyBuffer(lw, lr, make([]byte, 2048))
		ensureError(t, err)

		if got, want := nw, total; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}

		ensureError(t, lw.Close())
	})
}
