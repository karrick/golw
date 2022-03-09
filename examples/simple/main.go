package main // import "github.com/karrick/golw/examples/simple"

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/karrick/golw"
)

func main() {
	lw, err := golw.NewLogWriter(golw.Config{
		BufferBytes: 32,
		MaxBytes:    16,
	})
	if err != nil {
		bail(err)
	}
	fmt.Printf("lw: %#v\n", lw)

	for i := 0; i < 128; i++ {
		n, err := lw.Write([]byte{'.'})
		if err != nil {
			bail(err)
		}
		if n != 1 {
			bail(fmt.Errorf("GOT: %v; WANT: 1", n))
		}
	}

	if err = lw.Close(); err != nil {
		bail(err)
	}
}

func bail(err error) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
	os.Exit(1)
}
