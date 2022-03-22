package main // import "github.com/karrick/golw/benchmarks/lumberjack"

import (
	"flag"
	"io"
	"os"
	"path/filepath"

	"github.com/karrick/gonl"
	"github.com/natefinch/lumberjack"
)

func main() {
	optDest := flag.String("dir", ".", "optional directory for logs")
	flag.Parse()

	var wc io.WriteCloser
	var err error

	// Create a log writer.
	wc = &lumberjack.Logger{
		Filename: filepath.Join(*optDest, "lumberjack.log"),
		MaxSize:  1024, // 1024 MiB
	}

	// Log data needs to be written in chunks that end in newline
	// characters to prevent log lines from being split between log
	// files.
	wc = &gonl.PerLineWriter{WC: wc}

	if flag.NArg() == 0 {
		// Stream bytes from standard input.
		_, err = io.Copy(wc, os.Stdin)
		if err != nil {
			panic(err)
		}
	} else {
		buf := make([]byte, 32*1024) // size as io.Copy
		for _, arg := range flag.Args() {
			file, err := os.Open(arg)
			if err != nil {
				panic(err)
			}
			_, err = io.CopyBuffer(wc, file, buf)
			if err != nil {
				panic(err)
			}
			err = file.Close()
			if err != nil {
				panic(err)
			}
		}
	}

	// Close the log writer when done to flush lines to log files.
	if err = wc.Close(); err != nil {
		panic(err)
	}
}
