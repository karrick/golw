package benchmarks

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/karrick/golw"
	"github.com/karrick/gonl"
	"github.com/natefinch/lumberjack"
)

var tempdir string

//go:embed 2600-0.txt
var novel []byte

func setup() (string, error) {
	// return os.MkdirTemp("", "golw-benchmarks-")
	return "logs", nil
}

func teardown(tempdir string) error {
	return os.RemoveAll(tempdir)
}

func TestMain(m *testing.M) {
	flag.Parse()

	var code int // program exit code
	var err error

	// All tests use the same directory test scaffolding. Create the
	// directory hierarchy, run the tests, then remove the root
	// directory of the test scaffolding.

	tempdir, err = setup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "test setup: %s\n", err)
		code = 1
		return
	}

	defer func() {
		// if err := teardown(tempdir); err != nil {
		// 	fmt.Fprintf(os.Stderr, "test teardown: %s\n", err)
		// 	code = 1
		// }
		os.Exit(code)
	}()

	code = m.Run()
}

func benchmarkWriteCloser(b *testing.B, callback func() (io.WriteCloser, error)) {
	b.Helper()

	const limit = 1024 * 1024 // 1 MiB
	var r io.Reader
	buf := make([]byte, 32*1024) // same as io.Copy

	for i := 0; i < b.N; i++ {
		// For each iteration, create a data stream as shown
		// below. Note that because each LogWriter, Lumberjack, and
		// PerLineWriter are all initialized with a io.Writer, they
		// need to be created from the last to the first.
		//
		//     novel -> PerLineWriter -> (LogWriter | Lumberjack)

		// Fetch a log writer.
		wc, err := callback()
		if err != nil {
			b.Fatal(err)
		}

		// Mimic behavior of a logging library that invokes Write for
		// each and every single line written to the log writer.
		plw := &gonl.PerLineWriter{WC: wc}

		// Read from novel, but limited to the benchmark size.
		r = bytes.NewReader(novel[:limit])

		// Stream bytes from reader to log writer.
		nw, err := io.CopyBuffer(plw, r, buf)
		if err != nil {
			b.Fatal(err)
		}
		if got, want := nw, int64(limit); got != want {
			b.Errorf("GOT: %v; WANT: %v", got, want)
		}
		if err = plw.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGolw(b *testing.B) {
	// b.Skip()
	benchmarkWriteCloser(b, func() (io.WriteCloser, error) {
		cfg := &golw.Config{
			BaseNamePrefix: "golw",
			Directory:      tempdir,
			MaxBytes:       golw.Megabytes(1),
		}
		return golw.NewLogWriter(cfg)
	})
}

func BenchmarkLumberjack(b *testing.B) {
	// b.Skip()
	benchmarkWriteCloser(b, func() (io.WriteCloser, error) {
		lw := &lumberjack.Logger{
			Filename: filepath.Join(tempdir, "lumberjack.log"),
			MaxSize:  1, // 1 MiB
		}
		return lw, nil
	})
}
