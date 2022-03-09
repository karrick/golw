//go:build golw_debug
// +build golw_debug

package golw

import (
	"fmt"
	"os"
)

// debug formats and prints arguments to stderr for development builds
func debug(f string, a ...interface{}) {
	os.Stderr.Write([]byte("golw: " + fmt.Sprintf(f, a...)))
}
