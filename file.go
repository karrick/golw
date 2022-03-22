package golw

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

// closeLog closes file pointer to the log file.
func (lw *LogWriter) closeLog() error {
	debug("closeLog\n")
	err := lw.filePointer.Close()
	lw.filePointer = nil
	lw.fileSizeNow = 0
	return err
}

// openLog opens file pointer to log file for writing, creating the
// log file if it does not exist.
func (lw *LogWriter) openLog() error {
	debug("openLog\n")
	var err error
	lw.filePointer, err = os.OpenFile(lw.filePath,
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		lw.cfg.FileMode)
	if err != nil {
		return err
	}

	// Because the log file might already have some contents, check
	// its size and store it to prevent going over the configured max
	// log file size.
	st, err := lw.filePointer.Stat()
	if err != nil {
		// When cannot stat the open file pointer, close the file as
		// if it could not be opened.
		_ = lw.filePointer.Close()
		lw.filePointer = nil
		return err
	}

	// Store start size so know when to rotate.
	lw.fileSizeNow = st.Size()

	return nil
}

// renameLog renames the log file to a name that includes the
// timestamp of the first write written to it.
func (lw *LogWriter) renameLog() error {
	// timeStamp := lw.timeOfFirstWrite
	// if timeStamp == "" {
	// 	// Only happens when this is invoked multiple times without
	// 	// intervening write invocation.
	timeStamp := lw.cfg.TimeFormatter(time.Now())
	// TODO
	// }

	// Reset first write time so the next write stores the time it
	// took place.
	lw.timeOfFirstWrite = ""

	fileNameStamp := lw.cfg.BaseNamePrefix + "." + timeStamp + ".log"

	debug("renameLog: %s\n", fileNameStamp)

	filePathStamp := filepath.Join(lw.cfg.Directory, fileNameStamp)

	return os.Rename(lw.filePath, filePathStamp)
}

// rotateLog closes the open log file, renames it so it includes a
// timestamp in the file name, then creates a new log file. The
// timestamp it uses to rename the file is the string returned by the
// timestamp formatting callback function of the log rotator when
// invoked with the time recorded the first time that file was written
// to.
//
// TODO: Consider exporting this method, or one that flushes completed
// extents then rotateLogs.
func (lw *LogWriter) rotateLog() error {
	debug("rotateLog: having written %d bytes\n", lw.fileSizeNow)
	var err error

	// TODO: Consider renaming the existing log file before follow on
	// actions, because the current file pointer remains valid until
	// it is closed, even after the file it points to is renamed.

	if err = lw.closeLog(); err != nil {
		return err
	}

	if err = lw.renameLog(); err != nil {
		return err
	}

	return lw.openLog()
}

// writeBytes will write p to the open log file.
func (lw *LogWriter) writeBytes(p []byte) (int, error) {
	debug("writeBytes(%d bytes)\n", len(p))
	nw, err := lw.filePointer.Write(p)

	if nw < 0 || nw > len(p) {
		if err != nil {
			return nw, err
		}
		// NOTE: io.errInvalidWrite is not exported
		return nw, errors.New("invalid write result")
	}

	if nw < len(p) && err == nil {
		err = io.ErrShortWrite
	}

	lw.fileSizeNow += int64(nw)

	debug("writeBytes: fileSizeNow: %d\n", lw.fileSizeNow)

	return nw, err
}

func (lw *LogWriter) writeExtents(extentCount, byteCount int) (int, error) {
	debug("writeExtents(%d extents, %d bytes)\n", extentCount, byteCount)
	nw, err := lw.filePointer.Write(lw.buf[:byteCount])

	if nw < 0 || nw > byteCount {
		if err != nil {
			return 0, err
			// return nw, err
		}
		// NOTE: io.errInvalidWrite is not exported
		return 0, errors.New("invalid write result")
		// return nw, errors.New("invalid write result")
	}

	if nw < byteCount && err == nil {
		err = io.ErrShortWrite
	}

	lw.fileSizeNow += int64(nw)
	lw.buf = lw.buf[nw:]

	if err != nil {
		// Use the number of bytes written to determine which extents
		// were fully written, and which extent was partially written,
		// then update its length accordingly.
		extentCountMax := extentCount
		extentCount = 0
		for nw > 0 && extentCount < extentCountMax {
			nw -= lw.extents[extentCount]
			if nw < 0 {
				// nw = -5, means we have written 5 bytes of this
				// extent. Adding the negative value to the size of
				// the extent will subtract that length from the count
				// of bytes remaining to be written.
				lw.extents[extentCount] += nw
			} else {
				extentCount++
			}
		}
		// NOTE: Intentional fall through because same code.
	}

	lw.extents = lw.extents[extentCount:]

	debug("writeExtents: fileSizeNow: %d\n", lw.fileSizeNow)
	debug("writeExtents: extents remaining: %d\n", len(lw.extents))
	debug("writeExtents: bytes remaining: %d\n", len(lw.buf))

	return nw, err
}
