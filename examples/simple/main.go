package main // import "github.com/karrick/golw/examples/simple"

// Read from standard input, and writes to rotated logs.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/karrick/golw"
	"github.com/karrick/gonl"
)

func main() {
	lw, err := golw.NewLogWriter(nil)
	if err != nil {
		bail(err)
	}

	// This program streams data from standard input using io.Copy,
	// eliminating the likelihood that writes will always be sent to
	// the LogWriter on newline boundaries, as they otherwise might
	// when writing log files from a running application.
	//
	// golw.LogWriter always groups data in each Write together in the
	// same output file, and will not attempt to split the data in a
	// single Write call on newline boundaries.
	//
	// To prevent all data read from standard input from being written
	// to a single log file when newline characters are not guaranteed
	// to be the final character in the buffers sent from io.Copy, use
	// gonl.BatchLineWriter to split a single Write call into multiple
	// Write calls on newline boundaries.
	blw, err := gonl.NewBatchLineWriter(lw, 256)
	if err != nil {
		bail(err)
	}

	_, err = io.Copy(blw, os.Stdin)
	if err != nil {
		bail(err)
	}

	// NOTE: Closing gonl.BatchLineWriter closes the io.WriteCloser it
	// was created with, in this case, the golw.LogWriter.
	err = blw.Close()
	if err != nil {
		bail(err)
	}
}

func bail(err error) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
	os.Exit(1)
}
