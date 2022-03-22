package golw

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// NOTE: This library tracks how many bytes it writes to the open log
// file, but does not track whether any other process has written to
// the same file after this opens the file.

const (
	// DateTime is a time format string that represents current time
	// similar to time.RFC3339Nano, but with colons changed to
	// hyphens, and only three digits of precision for fractional
	// seconds. This is convenience constant for setting TimeFormat
	// to.
	DateTime = "2006-01-02T15-04-05.000Z0700"

	defaultBufferSizeMax = 128
	defaultMaxBytes      = 100 * (1 << 20) // 100 MiB
	defaultFileMode      = 0644
)

// Megabytes returns the number of bytes in the specified amount of
// megabytes.
func Megabytes(megabytes int) int64 { return int64(megabytes) * (1 << 20) }

// Config provides fields to customize behavior of a LogWriter.
type Config struct {
	// BaseNamePrefix is an optional prefix of the base name to use
	// when creating new output files inside the directory specified
	// by Directory. When this value is the empty string, the
	// LogWriter will use base name of os.Args[0].
	BaseNamePrefix string

	// BufferSizeMax is an optional size of a buffer to use between
	// writes. When this value is -1, the LogWriter will not buffer
	// writes, but will ensure log files are rotated when their size
	// would exceed MaxBytes. When this value is zero, the LogWriter
	// will use a buffer with a default size of 128 bytes. When this
	// value is greater than zero, the LogWriter will use a byte
	// buffer of this size to reduce the number of writes to the file
	// system.
	//
	// small value <-------------------------------------> large value
	// (more interactive)                           (less interactive)
	// (less efficient)                               (more efficient)
	BufferSizeMax int

	// Directory is an optional directory for creating new files. When
	// this value is the empty string, the LogWriter will use the
	// current working directory at the time the LogWriter was
	// created.
	Directory string

	// FileMode is an optional OS file mode to use when creating new
	// files. When this value is zero, the LogWriter will default to
	// 0644, which on UNIX, is equivalent to rw-r--r--.
	FileMode fs.FileMode

	// MaxBytes is an optional maximum number of bytes to write to any
	// particular log file. When a particular Write call sends a byte
	// slice longer than this value, the LogWriter will create a new
	// file to hold the contents of the entire byte slice, regardless
	// of its size, then will create a new output file the next time
	// its Write method is invoked.
	MaxBytes int64

	// TimeFormatter is an optional function that will format a given
	// time.Time value to a string in the desired time format for the
	// purpose of creating filenames with a timestamp. When this value
	// is the empty string, the value of TimeFormat is checked, and if
	// itself not empty, used to format the time.
	TimeFormatter func(time.Time) string

	// TimeFormat is an optional format to pass to time.Time's Format
	// method to format the current time when TimeFormatter is
	// empty. This value is ignored when TimeFormatter is not
	// nil. When TimeFormatter is nil and TimeFormat is the empty
	// string, the LogWriter uses UnixNano to format the time string
	// used to name rotated log files.
	TimeFormat string
}

func makeDateTimeFormatter(format string) func(time.Time) string {
	return func(t time.Time) string {
		return t.UTC().Format(format)
	}
}

func nanoDateTimeFormatter(t time.Time) string {
	return strconv.FormatInt(t.UTC().UnixNano(), 10)
}

// LogWriter is a io.WriteCloser that can act as the recipient of many
// logging libraries, and is designed to rotate log files at a
// specified size, and optionally buffer writes to reduce file system
// calls.
//
// NOTE: When LogWriter opens a previously created log file, it does
// not inspect its contents to determine the time of its first
// write. Therefore, later if it needs to rotate logs, it will rename
// the pre-existing log file with a timestamp of the first write
// applied to the log file after it was opened using this library.
type LogWriter struct {
	cfg     Config
	buf     []byte // buf stores all data to be written to file
	extents []int  // extents stores length of each newline terminated write

	// TODO: need to track write time for each extent, other wise new
	// files will be renamed with names of previously rotated files.
	writeTimes []time.Time

	timeOfFirstWrite  string
	filePath          string
	fileSizeNow       int64
	filePointer       *os.File
	waitingForNewline bool
}

// NewLogWriter returns a new LogWriter, or an error when the provided
// Config specifies disallowed argument values.
func NewLogWriter(cfg *Config) (*LogWriter, error) {
	var err error

	if cfg == nil {
		cfg = new(Config)
	}

	switch cfg.BufferSizeMax {
	case -1:
		cfg.BufferSizeMax = 0 // do not use in-memory buffering
	case 0:
		cfg.BufferSizeMax = defaultBufferSizeMax // default buffer size
	default:
		if cfg.BufferSizeMax < 0 {
			return nil, fmt.Errorf("cannot use negative flush threshold: %d", cfg.BufferSizeMax)
		}
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

	if cfg.MaxBytes == 0 {
		cfg.MaxBytes = defaultMaxBytes // default buffer size
	}

	if cfg.TimeFormatter == nil {
		if cfg.TimeFormat != "" {
			cfg.TimeFormatter = makeDateTimeFormatter(cfg.TimeFormat)
		} else {
			cfg.TimeFormatter = nanoDateTimeFormatter
		}
	}

	// Only file path and mode are needed prior to attempting to
	// create log file.
	lw := &LogWriter{
		cfg:      (*cfg),
		filePath: filepath.Join(cfg.Directory, cfg.BaseNamePrefix+".log"),
	}
	if err = lw.openLog(); err != nil {
		return nil, err
	}

	// The log file is open for writing in append mode. Populate
	// remainder of structure fields.
	if cfg.BufferSizeMax > 0 {
		lw.buf = make([]byte, 0, cfg.BufferSizeMax)
	}

	return lw, nil
}

// Close satisfies the io.Closer interface, and will flush and close
// the currently open log file, potentially returning an error
// resulting from flushing the buffer or closing the file. If the
// LogWriter was waiting to flush a line which was not newline
// terminated, it will be flushed as well, along with an appended
// newline character. This is done to prevent the next use of the log
// file from appending its first line to the middle of the previously
// written unterminated line.
func (lw *LogWriter) Close() error {
	debug("Close: buffer size: %d bytes\n", len(lw.buf))

	if len(lw.buf) > 0 {
		// Flush in-memory buffer before we close file.
		if lw.waitingForNewline {
			debug("Close: appending newline to complete the final extent\n")
			lw.buf = append(lw.buf, '\n')
			lw.extents[len(lw.extents)-1]++
			lw.waitingForNewline = false
		}
		if err := lw.flushCompletedExtents(); err != nil {
			// There is loss of data when cannot write everything.
			_ = lw.closeLog()
			return err
		}
	}

	return lw.closeLog()
}

// TODO: Consider exporting this method, or one similar to it.
func (lw *LogWriter) flushCompletedExtents() error {
	debug("flushCompletedExtents: extents: %d; bytes: %d\n", len(lw.extents), len(lw.buf))
	var err error

	// Loop through all of the completed extents waiting to be
	// written.
	for len(lw.extents) > 0 {
		if len(lw.extents) == 1 && lw.waitingForNewline {
			debug("flushCompletedExtents: single non terminated extent remains\n")
			// Nothing more can be written when a single incomplete
			// extent remains.
			break
		}
		if int64(lw.extents[0])+lw.fileSizeNow > lw.cfg.MaxBytes {
			debug("flushCompletedExtents: first extent too large for this log file\n")
			// Rotate the log file when the next extent will not fit
			// in the open log file.
			if lw.fileSizeNow > 0 {
				if err = lw.rotateLog(); err != nil {
					return err
				}
			}
			if int64(lw.extents[0]) > lw.cfg.MaxBytes {
				debug("flushCompletedExtents: first extent too large for empty log file\n")
				// This particular extent is too large to fit even in
				// its own log file. When this happens, put the data
				// in its own file, even if that file is larger than
				// max size.
				_, err = lw.writeExtents(1, lw.extents[0])
				if err != nil {
					return err
				}
				continue
			}
			// POST: Brand new log file has been opened.
		}
		if err = lw.flushAsMuchAsPossible(); err != nil {
			return err
		}
		debug("flushCompletedExtents: after flush, extents: %d; bytes: %d remains\n", len(lw.extents), len(lw.buf))
	}

	return nil
}

// flushAsMuchAsPossible will flush as many of the completed write
// extents as possible to the open log file without exceeding the
// configured maximum log file size, and without writing an extent
// that is not newline terminated.
func (lw *LogWriter) flushAsMuchAsPossible() error {
	debug("flushAsMuchAsPossible: extents: %d; len(lw.buf): %d\n", len(lw.extents), len(lw.buf))
	// Determine how much data can be flushed to the open log file..
	flushExtentCountMax := len(lw.extents)
	if lw.waitingForNewline {
		// Decrement the number of write extents that may be written
		// when the final write extent is not newline terminated.
		debug("flushAsMuchAsPossible: final extent waiting for newline\n")
		flushExtentCountMax--
	}

	if flushExtentCountMax == 0 {
		debug("flushAsMuchAsPossible: no extents to flush\n")
		// There is nothing to do when either there are no extents in
		// the buffer, or there is a single extent but it is not
		// newline terminated.
		return nil
	}

	// Determine how many extents may be flushed to the open log file
	// before rotation based on configured file size limit and the
	// size of each successive extent.
	bytesRemaining := lw.cfg.MaxBytes - lw.fileSizeNow

	var flushByteCount int64
	var flushExtentCount int

	for flushExtentCount = 0; flushExtentCount < flushExtentCountMax; flushExtentCount++ {
		fbc := flushByteCount + int64(lw.extents[flushExtentCount])
		if fbc > bytesRemaining {
			break // this extent will not fit in the open log file
		}
		flushByteCount = fbc
	}

	// All extents strictly less than flushExtentCount may be flushed
	// before log file needs to be rotated. Flushing them will add
	// flushByteCount bytes to that log file.

	if flushExtentCount == 0 {
		debug("flushAsMuchAsPossible: flushing the first extent would exceed log file max bytes\n")
		// Flushing the first extent would cause open log file to
		// exceed max bytes.
		return nil
	}

	debug("flushAsMuchAsPossible: can flush %d extents with %d bytes\n", flushExtentCount, flushByteCount)

	// Flush as much as the buffer as possible to open log file such
	// that it will not exceed max bytes.
	_, err := lw.writeExtents(flushExtentCount, int(flushByteCount))
	return err
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
	if lw.timeOfFirstWrite == "" {
		// Store the current time when this particular log file has
		// yet to be written to. Later, when renaming the log file
		// with a timestamp, will use this recorded time in the file
		// name for the renamed log file.
		lw.timeOfFirstWrite = lw.cfg.TimeFormatter(time.Now())
		debug("time of first write: %q\n", lw.timeOfFirstWrite)
	}

	if lw.cfg.BufferSizeMax > 0 {
		// Buffer the writes through the in-memory buffer when
		// configured.
		debug("Write(%d bytes): buffer has %d out of %d filled\n", len(p), len(lw.buf), lw.cfg.BufferSizeMax)

		if len(lw.buf) > 0 && len(lw.buf)+len(p) > lw.cfg.BufferSizeMax {
			debug("Write: p will not fit in non-empty buffer\n")
			// Once a Write triggers having to flush the buffer, might
			// as well flush as much as possible to one or more files.

			if len(lw.extents) > 1 || !lw.waitingForNewline {
				if err = lw.flushCompletedExtents(); err != nil {
					return 0, err
				}
			}
		}

		// The in-memory buffer is as empty as it can get before we
		// write p to it.

		if lw.waitingForNewline {
			debug("Write: appending to previous extent\n")
			// Append this to previous write extent without modifying
			// its time when previous write was not terminated with a
			// newline.
			lw.extents[len(lw.extents)-1] += len(p)
		} else {
			debug("Write: creating new extent\n")
			// Create and append a new write extent when previous
			// write was terminated with newline.
			lw.extents = append(lw.extents, len(p))
		}

		// Append p to the buffer, and remember whether this write was
		// terminated with a newline for use during next write.
		debug("Write: appended %d bytes to buffer\n", len(p))
		lw.buf = append(lw.buf, p...)
		lw.waitingForNewline = p[len(p)-1] != '\n'
		debug("Write: final byte is newline: %t\n", !lw.waitingForNewline)

		return len(p), nil
	}

	// Write p to disk when not configured for in-memory buffering.
	debug("Write(%d bytes): not using buffer\n", len(p))

	if lw.fileSizeNow > 0 && lw.fileSizeNow+int64(len(p)) > lw.cfg.MaxBytes {
		debug("Write: p will not fit in open log file\n")
		// Rotate the open log file when it does not have enough room
		// to hold the contents of p.
		if err = lw.rotateLog(); err != nil {
			return 0, err
		}
	}

	return lw.writeBytes(p)
}
