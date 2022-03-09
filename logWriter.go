package golw

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	// DateTime is a time format string that represents current time
	// similar to time.RFC3339Nano, but with colons changed to
	// hyphens, and only three digits of precision for fractional
	// seconds. This is convenience constant for setting TimeFormat
	// to.
	DateTime = "2006-01-02T15-04-05.000Z0700"

	defaultMaxBytes = 100 * (1 << 20) // 100 MiB
	defaultFileMode = 0644
)

type Config struct {
	// BaseNamePrefix is the prefix of the base name to use when
	// creating new output files inside the directory specified by
	// Directory. When this value is the empty string, the LogWriter
	// will use base name of os.Args[0].
	BaseNamePrefix string

	// BufferBytes is the size of a buffer to use between writes. When
	// this value is zero, the LogWriter will not buffer any writes,
	// and will only ensure log files are rotated when their size
	// would exceed MaxBytes. When this value is greater than zero,
	// the LogWriter will only write to the current log file when the
	// buffer size exceeds this value.
	BufferBytes int

	// Directory is the directory for creating new files. When this
	// value is the empty string, the LogWriter will use the current
	// working directory at the time the LogWriter was created.
	Directory string

	// FileMode is the OS file mode to use when creating new
	// files. When this value is zero, the LogWriter will default to
	// 0644, which on UNIX, is equivalent to rw-r--r--.
	FileMode fs.FileMode

	// MaxBytes is the maximum number of bytes to write to any
	// particular file. When a particular byte slice is longer than
	// this value, the LogWriter will create a new file to hold the
	// contents of the entire byte slice, regardless of its size, then
	// will create a new output file the next time its Write method is
	// invoked.
	MaxBytes int64

	// TimeFormatter is the function that will format a given
	// time.Time value to a string in the desired time format for the
	// purpose of creating filenames with a timestamp. When this value
	// is the empty string, the value of TimeFormat is checked, and if
	// itself not empty, used to format the time.
	TimeFormatter func(time.Time) string

	// TimeFormat is the format to pass to time.Time's Format method
	// to format the current time when TimeFormatter is empty. This
	// value is ignored when TimeFormatter is not nil. When
	// TimeFormatter is nil and TimeFormat is the empty string, the
	// LogWriter uses UnixNano to format the time string used to name
	// rotated log files.
	TimeFormat string
}

type LogWriter struct {
	buf []byte

	baseNamePrefix string
	directory      string
	filePath       string
	timeFormat     string

	maxBytes     int64
	writtenBytes int64

	timeFormatter func(time.Time) string
	filePointer   *os.File

	flushThreshold      int
	indexOfFinalNewline int

	fileMode fs.FileMode
}

func NewLogWriter(cfg Config) (*LogWriter, error) {
	var err error

	if cfg.BufferBytes < 0 {
		return nil, fmt.Errorf("cannot use negative flush threshold: %d", cfg.BufferBytes)
	}
	if cfg.Directory == "" {
		cfg.Directory, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("cannot determine working directory: %w", err)
		}
	}

	if cfg.BaseNamePrefix == "" {
		cfg.BaseNamePrefix = filepath.Base(os.Args[0])
	}
	if cfg.FileMode == 0 {
		cfg.FileMode = defaultFileMode
	}
	if cfg.MaxBytes < 1 {
		cfg.MaxBytes = defaultMaxBytes
	}
	if cfg.TimeFormatter == nil {
		if cfg.TimeFormat != "" {
			cfg.TimeFormatter = makeDateTimeFormatter(cfg.TimeFormat)
		} else {
			cfg.TimeFormatter = nanoDateTimeFormatter
		}
	}

	lw := &LogWriter{
		fileMode: cfg.FileMode,
		filePath: filepath.Join(cfg.Directory, cfg.BaseNamePrefix+".log"),
	}

	// Only file path and mode are needed prior to attempting to
	// create log file.
	if err = lw.createLogFile(); err != nil {
		return nil, err
	}

	// Log file is open for writing. Populate remainder of structure
	// fields.
	lw.baseNamePrefix = cfg.BaseNamePrefix
	lw.directory = cfg.Directory
	lw.indexOfFinalNewline = -1
	lw.maxBytes = cfg.MaxBytes
	lw.timeFormatter = cfg.TimeFormatter

	if cfg.BufferBytes > 0 {
		lw.flushThreshold = cfg.BufferBytes
		lw.buf = make([]byte, 0, cfg.BufferBytes)
	}

	return lw, nil
}

// Close satisfies the io.Closer interface, and will flush and close
// the currently open log file, potentially returning an error
// resulting from flushing the buffer to or closing the file.
func (lw *LogWriter) Close() error {
	var writeErr, closeErr error

	if len(lw.buf) > 0 {
		// Flush buf before we close file.
		_, writeErr = lw.filePointer.Write(lw.buf)
		lw.buf = lw.buf[:0]
	}

	lw.indexOfFinalNewline = -1

	closeErr = lw.closeLogFile()

	// Error from Write is more significant than error from Close.
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// Write satisfies the io.Writer interface, allowing a program to
// write byte slices to the LogWriter. When the combined size of the
// current log file and the size of the provided byte slice is larger
// than the LogWriter's MaxBytes config argument, TODO. When a
// particular byte slice is longer than the LogWriter's MaxBytes
// config argument, the LogWriter will create a new file to hold the
// contents of the entire byte slice, regardless of its size, then
// will create a new output file the next time Write is invoked. Any
// time the LogWriter receives an error while attempting to roll the
// underlying output file, it simply writes the byte slice to the
// existing underlying file.
func (lw *LogWriter) Write(p []byte) (written int, err error) {
	// OBSERVATIONS:
	//
	//   1. When caller provides more than a single line of text, the
	//   provided lines should remain together in the same log file.
	//
	//   2. Optimize for most common case when p will fit in the
	//   current file.

	if lw.flushThreshold == 0 {
		if total := lw.writtenBytes + int64(len(p)); total > lw.maxBytes {
			if err = lw.rotate(); err != nil {
				return 0, err
			}
		}
		n, err := lw.filePointer.Write(p)
		lw.writtenBytes += int64(n)
		return n, err
	}

	origBufLen := len(lw.buf)
	lw.buf = append(lw.buf, p...)

	if index := bytes.LastIndexByte(p, '\n'); index >= 0 {
		lw.indexOfFinalNewline = origBufLen + index
	}

	if len(lw.buf) <= lw.flushThreshold || lw.indexOfFinalNewline < 0 {
		// Either do not need to flush, or no newline in buffer yet.
		return len(p), nil
	}

	debug("need to flush: buffer len: %d; newline index: %d\n",
		len(lw.buf), lw.indexOfFinalNewline)

	// Buffer is larger than threshold and has at least one LF: write
	// everything up to and including that final LF.
	return lw.flush(origBufLen, len(p), lw.indexOfFinalNewline+1)
}

// flush will flush the buffer to file pointer, up to and including
// specified index.
func (lw *LogWriter) flush(olen, dlen, index int) (int, error) {
	var err error

	count := int64(index)
	potentialSize := lw.writtenBytes + count

	if potentialSize > lw.maxBytes || count > lw.maxBytes {
		debug("need to rotate output file: %d; %d\n", potentialSize, lw.maxBytes)
		if err = lw.rotate(); err != nil {
			return 0, err
		}
		// POST: File pointer has a brand new output file.
	}

	nw, err := lw.filePointer.Write(lw.buf[:index])
	if nw < 0 || nw > index {
		if err == nil {
			err = fmt.Errorf("invalid write result: %d", nw)
		}
		nw = 0
	} else if nw > 0 {
		// Expected path
		lw.writtenBytes += int64(nw)
		nc := copy(lw.buf, lw.buf[nw:])
		lw.buf = lw.buf[:nc]
	}
	if err == nil {
		lw.indexOfFinalNewline -= nw
		return dlen, nil
	}
	// Writing to file returned an error. How many bytes of the new
	// data were written to the file?
	nb := nw - olen
	if nb < 0 {
		lw.buf = lw.buf[:-nb]
		nb = 0
	} else {
		lw.buf = lw.buf[:0]
	}
	lw.indexOfFinalNewline = bytes.LastIndexByte(lw.buf, '\n')
	return nb, err
}

const fileFlags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC | os.O_APPEND

// createLogFile opens file pointer to log file for writing. If the
// log file already exists, it truncates its contents. If the log file
// does not exist, it creates it. It always returns either nil error
// or *os.PathError.
func (rot *LogWriter) createLogFile() error {
	var err error
	rot.filePointer, err = fileOpen(rot.filePath, fileFlags, rot.fileMode)
	return err
}

// closeLogFile closes file pointer to the log file. It always returns
// either nil error or *os.PathError.
func (rot *LogWriter) closeLogFile() error {
	err := fileClose(rot.filePath, rot.filePointer)
	rot.filePointer = nil
	rot.writtenBytes = 0
	return err
}

// renameLogFile renames the log file to a name that includes the a
// timestamp. It always returns either nil error or *os.LinkError.
func (rot *LogWriter) renameLogFile() error {
	timeStamp := rot.timeFormatter(time.Now())
	fileNameStamp := rot.baseNamePrefix + "." + timeStamp + ".log"
	filePathStamp := filepath.Join(rot.directory, fileNameStamp)
	return fileRename(rot.filePath, filePathStamp)
}

// rotate closes the current log file, renames it so it includes a
// timestamp in the file name, then creates a new log file. The
// timestamp it uses to rename the file is the string returned by the
// timestamp formatting callback function of the log rotator when
// invoked with the current time.
func (rot *LogWriter) rotate() error {
	// TODO: consider exporting this method.
	var err error
	if err = rot.closeLogFile(); err != nil {
		return err
	}
	if err = rot.renameLogFile(); err != nil {
		return err
	}
	return rot.createLogFile()
}

func makeDateTimeFormatter(format string) func(time.Time) string {
	return func(t time.Time) string {
		return t.UTC().Format(format)
	}
}

func nanoDateTimeFormatter(t time.Time) string {
	return strconv.FormatInt(t.UTC().UnixNano(), 10)
}
