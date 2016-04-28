package files

import (
	"errors"
	"io"
	"os"
)

var (
	ErrNotDirectory = errors.New("Couln't call NextFile(), this isn't a directory")
	ErrNotReader    = errors.New("This file is a directory, can't use Reader functions")
)

// An AdvReader is like a Reader but supports getting the current file
// path and offset into the file when applicable.
type AdvReader interface {
	io.Reader
	Offset() int64
	AbsPath() string
}

type advReaderAdapter struct {
	io.Reader
}

func (advReaderAdapter) Offset() int64 {
	return -1
}

func (advReaderAdapter) AbsPath() string {
	return ""
}

func AdvReaderAdapter(r io.Reader) AdvReader {
	switch t := r.(type) {
	case AdvReader:
		return t
	default:
		return advReaderAdapter{r}
	}
}

// File is an interface that provides functionality for handling
// files/directories as values that can be supplied to commands. For
// directories, child files are accessed serially by calling `NextFile()`.
type File interface {
	// Files implement ReadCloser, but can only be read from or closed if
	// they are not directories
	io.ReadCloser
	Offset() int64

	// FileName returns a filename path associated with this file
	FileName() string

	// FullPath returns the full path in the os associated with this file
	FullPath() string

	// AbsPath returns the absolute path, not necessary unique
	AbsPath() string

	// IsDirectory returns true if the File is a directory (and therefore
	// supports calling `NextFile`) and false if the File is a normal file
	// (and therefor supports calling `Read` and `Close`)
	IsDirectory() bool

	// NextFile returns the next child file available (if the File is a
	// directory). It will return (nil, io.EOF) if no more files are
	// available. If the file is a regular file (not a directory), NextFile
	// will return a non-nil error.
	NextFile() (File, error)
}

type StatFile interface {
	File

	Stat() os.FileInfo
}

type PeekFile interface {
	SizeFile

	Peek(n int) File
	Length() int
}

type SizeFile interface {
	File

	Size() (int64, error)
}
