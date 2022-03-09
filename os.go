package golw

import "os"

// fileClose closes the File, rendering it unusable for I/O.  On files
// that support SetDeadline, any pending I/O operations will be
// canceled and return immediately with an error.  Close will return
// an error if it has already been called.
func fileClose(pathname string, fp *os.File) error {
	err := fp.Close()
	switch err.(type) {
	case nil:
		return nil
	case *os.PathError:
		return err
	default:
		return &os.PathError{Op: "close", Path: pathname, Err: err}
	}
}

// fileOpen opens the named file with specified flag (O_RDONLY
// etc.). If the file does not exist, and the O_CREATE flag is passed,
// it is created with mode perm (before umask). If successful, methods
// on the returned File can be used for I/O.  If there is an error, it
// will be of type *PathError.
func fileOpen(pathname string, flags int, mode os.FileMode) (*os.File, error) {
	fp, err := os.OpenFile(pathname, flags, mode)
	switch err.(type) {
	case nil:
		return fp, nil
	case *os.PathError:
		return nil, err
	default:
		return nil, &os.PathError{Op: "open", Path: pathname, Err: err}
	}
}

// fileRename renames (moves) oldpath to newpath.  If newpath already
// exists and is not a directory, Rename replaces it.  OS-specific
// restrictions may apply when oldpath and newpath are in different
// directories.  If there is an error, it will be of type *LinkError.
func fileRename(oldPath, newPath string) error {
	err := os.Rename(oldPath, newPath)
	switch err.(type) {
	case nil:
		return nil
	case *os.LinkError:
		return err
	default:
		return &os.LinkError{
			Op:  "rename",
			Old: oldPath,
			New: newPath,
			Err: err,
		}
	}
}
