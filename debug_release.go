//go:build !golw_debug
// +build !golw_debug

package golw

// debug is a no-op for release builds
func debug(_ string, _ ...interface{}) {}
