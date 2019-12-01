/*
Copyright (c) 2019 Maxim Konakov
All rights reserved.

Redistribution and use in source and binary forms, with or without modification,
are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice,
   this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.
3. Neither the name of the copyright holder nor the names of its contributors
   may be used to endorse or promote products derived from this software without
   specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING,
BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY
OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE,
EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

/*
Package stout (STream OUTput): writing byte streams in a type-safe and extensible way.
*/
package stout

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Chunk is a function that writes data using the given stream writer,
// returning the total number of bytes written, or an error. A number of constructors
// of chunk functions is provided by this package, though typically such functions are
// written by the user.
type Chunk = func(*Writer) (int64, error)

// Stream is a container for stout.Writer that provides Write() function.
// Stream objects are lightweight and can be passed around with low overhead.
type Stream struct {
	w *Writer
}

// Write does the actual writing to the stream, checking errors and also
// flushing and closing the underlying writer as necessary.
func (s Stream) Write(chunks ...Chunk) (n int64, err error) {
	if s.w.close != nil {
		defer func() {
			if e := s.w.close(); e != nil && err == nil {
				err = e
			}
		}()
	}

	if n, err = s.w.WriteChunks(chunks); err == nil && s.w.flush != nil {
		err = s.w.flush()
	}

	return
}

/*
Writer implements the following interface:

	io.Writer
	io.ByteWriter
	io.StringWriter
	io.ReaderFrom

	WriteRune(rune) (int, error)
	WriteChunks([]Chunk) (int64, error)
*/
type Writer struct {
	writeByteSlice func([]byte) (int, error)      // required, must not be nil
	writeByte      func(byte) error               // required, must not be nil
	writeRune      func(rune) (int, error)        // required, must not be nil
	writeString    func(string) (int, error)      // required, must not be nil
	readFrom       func(io.Reader) (int64, error) // required, must not be nil
	flush          func() error                   // optional, may be nil
	close          func() error                   // optional, may be nil
}

// WriterStream constructs a stream from the given io.Writer object.
func WriterStream(w io.Writer) Stream {
	s := &Writer{writeByteSlice: w.Write}

	// inspect the writer to find implementations of the required functions
	// 1. WriteByte
	if wr, ok := w.(io.ByteWriter); ok {
		s.writeByte = wr.WriteByte
	} else {
		s.writeByte = func(b byte) (err error) {
			m := [1]byte{b}
			_, err = w.Write(m[:])
			return
		}
	}

	// 2. WriteRune
	type runeWriter interface{ WriteRune(rune) (int, error) }

	if wr, ok := w.(runeWriter); ok {
		s.writeRune = wr.WriteRune
	} else {
		s.writeRune = func(r rune) (int, error) {
			var b [utf8.UTFMax]byte

			return w.Write(b[:utf8.EncodeRune(b[:], r)])
		}
	}

	// 3. WriteString
	if wr, ok := w.(io.StringWriter); ok {
		s.writeString = wr.WriteString
	} else {
		s.writeString = func(str string) (int, error) { return w.Write([]byte(str)) }
	}

	// 4. ReadFrom
	if wr, ok := w.(io.ReaderFrom); ok {
		s.readFrom = wr.ReadFrom
	} else {
		s.readFrom = func(src io.Reader) (int64, error) { return io.Copy(w, src) }
	}

	// 5. Flush
	type flusher interface{ Flush() error }

	if wr, ok := w.(flusher); ok {
		s.flush = wr.Flush
	}

	return Stream{s}
}

// WriteCloserStream constructs a stream from the given io.WriteCloser object.
// The writer object will be closed upon exit from the stream Write() function.
func WriteCloserStream(w io.WriteCloser) (s Stream) {
	s = WriterStream(w)
	s.w.close = w.Close
	return
}

// WriterBufferedStream constructs a stream from the given io.Writer object,
// with bufio.Writer buffer on top of it.
func WriterBufferedStream(w io.Writer) Stream {
	b := bufio.NewWriter(w)

	return Stream{
		&Writer{
			writeByteSlice: b.Write,
			writeByte:      b.WriteByte,
			writeRune:      b.WriteRune,
			writeString:    b.WriteString,
			readFrom:       b.ReadFrom,
			flush:          b.Flush,
		},
	}
}

// WriteCloserBufferedStream constructs a stream from the given io.WriteCloser object,
// with bufio.Writer buffer on top of it. The writer object will be closed upon
// exit from the stream Write() function.
func WriteCloserBufferedStream(w io.WriteCloser) (s Stream) {
	s = WriterBufferedStream(w)
	s.w.close = w.Close
	return
}

// ByteBufferStream constructs a stream that writes to the given bytes.Buffer object.
func ByteBufferStream(b *bytes.Buffer) Stream {
	return Stream{
		&Writer{
			writeByteSlice: b.Write,
			writeByte:      b.WriteByte,
			writeRune:      b.WriteRune,
			writeString:    b.WriteString,
			readFrom:       b.ReadFrom,
		},
	}
}

// StringBuilderStream constructs a stream that writes to the given strings.Builder object.
func StringBuilderStream(b *strings.Builder) Stream {
	return Stream{
		&Writer{
			writeByteSlice: b.Write,
			writeByte:      b.WriteByte,
			writeRune:      b.WriteRune,
			writeString:    b.WriteString,
			readFrom:       func(src io.Reader) (int64, error) { return io.Copy(b, src) },
		},
	}
}

// Write implements io.Writer interface.
func (w *Writer) Write(s []byte) (n int, err error) {
	if len(s) > 0 {
		n, err = w.writeByteSlice(s)
	}

	return
}

// WriteByte implements io.ByteWriter interface.
func (w *Writer) WriteByte(b byte) error { return w.writeByte(b) }

// WriteRune writes the given rune to the stream.
func (w *Writer) WriteRune(r rune) (int, error) { return w.writeRune(r) }

// WriteString implements io.StringWriter interface.
func (w *Writer) WriteString(s string) (n int, err error) {
	if len(s) > 0 {
		n, err = w.writeString(s)
	}

	return
}

// ReadFrom implements io.ReaderFrom interface.
func (w *Writer) ReadFrom(r io.Reader) (int64, error) { return w.readFrom(r) }

// WriteChunks writes the given chunks to the stream. Useful when implementing a chunk
// composed from other chunks.
func (w *Writer) WriteChunks(chunks []Chunk) (n int64, err error) {
	for i, fn := range chunks {
		var m int64

		if m, err = fn(w); err != nil {
			err = fmt.Errorf("writing stream chunk %d: %w", i, err) // chunks counted from 0
			break
		}

		n += m
	}

	return
}

// Copy the data from the given source, and then close the input stream.
func (w *Writer) readFromAndClose(src io.ReadCloser) (n int64, err error) {
	defer func() {
		if e := src.Close(); e != nil && err == nil {
			err = e
		}
	}()

	n, err = w.ReadFrom(src)
	return
}

// All constructs a sequential composition of the given chunks.
func All(chunks ...Chunk) Chunk {
	return func(w *Writer) (int64, error) {
		return w.WriteChunks(chunks)
	}
}

// Join constructs a chunk function that writes the given chunks with the specified
// separator between them.
func Join(sep string, chunks ...Chunk) Chunk {
	switch len(chunks) {
	case 0:
		return nopChunk
	case 1:
		return chunks[0]
	}

	if len(sep) == 0 {
		return All(chunks...)
	}

	sc := String(sep)
	list := append(make([]Chunk, 0, 2*len(chunks)-1), chunks[0])

	for _, c := range chunks[1:] {
		list = append(append(list, sc), c)
	}

	return All(list...)
}

// Repeat constructs a chunk function that calls the given function over and over
// again until it returns a non-nil error. The supplied function serves the same
// purpose as a regular chunk function. The first parameter to each call is the
// call's number (counting up from 0), it may help to decide when to stop the iteration.
// The supplied function is expected to return io.EOF to stop the iteration without an error.
func Repeat(fn func(int, *Writer) (int64, error)) Chunk {
	return func(w *Writer) (n int64, err error) {
		var m int64
		var i int

		for m, err = fn(i, w); err == nil; m, err = fn(i, w) {
			n += m
			i++
		}

		if err == io.EOF {
			n += m
			err = nil
		}

		return
	}
}

// RepeatN constructs a chunk function that calls the given chunk the specified number of times.
func RepeatN(num int, chunk Chunk) Chunk {
	if num <= 0 {
		return nopChunk
	}

	return Repeat(func(i int, w *Writer) (int64, error) {
		if i < num {
			return chunk(w)
		}

		return 0, io.EOF
	})
}

// ByteSlice constructs a chunk function that writes the given byte slice to a stream.
func ByteSlice(val []byte) Chunk {
	if len(val) == 0 {
		return nopChunk
	}

	return func(w *Writer) (int64, error) {
		n, err := w.Write(val)
		return int64(n), err
	}
}

// String constructs a chunk function that writes the given string to a stream.
func String(val string) Chunk {
	if len(val) == 0 {
		return nopChunk
	}

	return func(w *Writer) (int64, error) {
		n, err := w.WriteString(val)
		return int64(n), err
	}
}

// no-op stream write
func nopChunk(_ *Writer) (int64, error) {
	return 0, nil
}

// Byte constructs a chunk function that writes the given byte to a stream.
func Byte(val byte) Chunk {
	return func(w *Writer) (n int64, err error) {
		if err = w.WriteByte(val); err == nil {
			n = 1
		}

		return
	}
}

// Rune constructs a chunk function that writes the given rune to a stream.
func Rune(val rune) Chunk {
	return func(w *Writer) (int64, error) {
		n, err := w.WriteRune(val)
		return int64(n), err
	}
}

// Reader constructs a chunk function that copies data from the given io.Reader to a stream.
func Reader(src io.Reader) Chunk {
	return func(w *Writer) (int64, error) {
		return w.ReadFrom(src)
	}
}

// ReadCloser constructs a chunk function that copies data from the given io.ReadCloser to a stream,
// also closing the reader upon completion.
func ReadCloser(src io.ReadCloser) Chunk {
	return func(w *Writer) (int64, error) {
		return w.readFromAndClose(src)
	}
}

// File constructs a chunk function that copies data from the given disk file to a stream.
func File(pathname string) Chunk {
	return func(w *Writer) (n int64, err error) {
		var file *os.File

		if file, err = os.Open(pathname); err == nil {
			n, err = w.readFromAndClose(file)
		}

		return
	}
}

// Command constructs a chunk function that invokes the given command and copies its STDOUT
// to a stream. The initial 2048 bytes of the command's STDERR output (if any) are recorded
// and returned as an error message if the command fails with a non-zero exit code.
func Command(name string, args ...string) Chunk {
	return cmdChunk(exec.Command(name, args...))
}

// CommandContext is like Command, but also takes a context which when becomes done terminates
// the process.
func CommandContext(ctx context.Context, name string, args ...string) Chunk {
	return cmdChunk(exec.CommandContext(ctx, name, args...))
}

func cmdChunk(cmd *exec.Cmd) Chunk {
	return func(w *Writer) (n int64, err error) {
		// set stderr
		stderr := limitedWriter{limit: 2048}

		cmd.Stderr = &stderr

		// get stdout pipe
		var stdout io.ReadCloser

		if stdout, err = cmd.StdoutPipe(); err != nil {
			return
		}

		// start the command
		if err = cmd.Start(); err != nil {
			return
		}

		// read output
		if n, err = w.ReadFrom(stdout); err != nil {
			// this error may come from the target writer, so in order to make the command fail
			// here we can just close the STDOUT pipe (is that correct?)
			stdout.Close()
			cmd.Wait()

			return
		}

		// wait for completion
		if err = cmd.Wait(); err != nil {
			if msg := stderr.String(); len(msg) > 0 {
				err = errors.New(msg)
			} else {
				err = fmt.Errorf("command %q: %w", cmd.Args[0], err)
			}
		}

		return
	}
}

type limitedWriter struct {
	b     []byte
	limit int
}

func (w *limitedWriter) Write(s []byte) (int, error) {
	if n := min(w.limit-len(w.b), len(s)); n > 0 {
		w.b = append(w.b, s[:n]...)
	}

	return len(s), nil
}

func (w *limitedWriter) String() string {
	// truncation may result in broken UTF-8 encoding at the end of the message
	s := w.b

	for r, n := utf8.DecodeLastRune(s); r == utf8.RuneError && n > 0; r, n = utf8.DecodeLastRune(s) {
		s = s[:len(s)-n]
	}

	// trim space and return as a string
	return string(bytes.TrimSpace(s))
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

// WriteFile is a convenience function for writing to the given disk file. Existing file gets overwritten.
func WriteFile(pathname string, perm os.FileMode, chunks ...Chunk) (int64, error) {
	return writeFile(pathname, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm, chunks)
}

// AppendToFile is a convenience function for appending data to the given disk file. The file is created
// if does not exist.
func AppendToFile(pathname string, perm os.FileMode, chunks ...Chunk) (int64, error) {
	return writeFile(pathname, os.O_CREATE|os.O_WRONLY|os.O_APPEND, perm, chunks)
}

// write to disk file
func writeFile(pathname string, flags int, perm os.FileMode, chunks []Chunk) (n int64, err error) {
	var file *os.File

	if file, err = os.OpenFile(pathname, flags, perm|0600); err == nil {
		n, err = WriteCloserBufferedStream(file).Write(chunks...)
	}

	return
}

// AtomicWriteFile is a convenience function for writing to the given disk file. The data are first written
// to a temporary file, and the target file (if exists) gets overwritten only upon successful completion
// of the write operation. In case of any error the temporary is removed from the disk, and the target
// is left unmodified.
func AtomicWriteFile(pathname string, perm os.FileMode, chunks ...Chunk) (n int64, err error) {
	// create temporary file in the same directory as the target
	var fd *os.File

	if fd, err = ioutil.TempFile(filepath.Dir(pathname), "tmp-"); err != nil {
		return
	}

	temp := fd.Name()

	// make sure the temporary file is removed on failure
	defer func() {
		if p := recover(); p != nil {
			os.Remove(temp)
			panic(p)
		}

		if err != nil {
			n = 0
			os.Remove(temp)
		}
	}()

	// do the write
	if n, err = WriteCloserBufferedStream(fd).Write(chunks...); err == nil {
		err = os.Rename(temp, pathname)
	}

	return
}

// WriteTempFile writes the given chunks to a temporary file and returns the full path to
// the file and the number of bytes written, or an error. In case of any error or a panic
// the temporary file is removed from the disk. The file name has prefix "tmp-", and it is
// located in the default directory for temporary files (see os.TempDir).
func WriteTempFile(chunks ...Chunk) (name string, n int64, err error) {
	var fd *os.File

	if fd, err = ioutil.TempFile("", "tmp-"); err != nil {
		return
	}

	name = fd.Name()

	// make sure the temporary file is removed on failure
	defer func() {
		if p := recover(); p != nil {
			os.Remove(name)
			panic(p)
		}

		if err != nil {
			os.Remove(name)
			name = ""
			n = 0
		}
	}()

	// do the write
	n, err = WriteCloserBufferedStream(fd).Write(chunks...)

	return
}
