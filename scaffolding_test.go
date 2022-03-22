package golw

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

var tempdir string

func setup() (string, error) {
	// return os.MkdirTemp("", "golw-")
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
