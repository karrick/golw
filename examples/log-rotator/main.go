package main // import "github.com/karrick/golw/examples/log-rotator"

// Read from standard input, and writes to rotated logs.

import (
	"fmt"
	"io"
	"os"

	"github.com/karrick/golw"
)

const (
	progname    = "log-rotator"
	copyBufSize = 32768
	logBufSize  = 32768
)

func main() {
	cfg := golw.Config{
		BufferBytes: logBufSize,
		MaxBytes:    1 * (1 << 20), // 100 MiB
	}

	lw, err := golw.NewLogWriter(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err)
		os.Exit(1)
	}

	buf := make([]byte, copyBufSize)

	if len(os.Args) == 1 {
		_, err = io.CopyBuffer(lw, os.Stdin, buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err)
		}
	} else {
		for _, name := range os.Args[1:] {
			fp, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err)
				continue
			}

			_, err = io.CopyBuffer(lw, fp, buf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err)
			}

			err = fp.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err)
			}
		}
	}

	err = lw.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err)
	}
}
