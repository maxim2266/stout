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

package stout

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBasics(t *testing.T) {
	const str = "Hello, world!!!"

	var s strings.Builder

	n, err := StringBuilderStream(&s).Write(
		String("Hello"),
		Rune(','),
		Byte(' '),
		ByteSlice([]byte("world")),
		RepeatN(3, Byte('!')),
	)

	if err != nil {
		t.Error(err)
		return
	}

	if m := int64(len(str)); n != m {
		t.Errorf("Unexpected number of bytes written: %d instead of %d", n, m)
		return
	}

	if res := s.String(); res != str {
		t.Errorf("Unexpected result: %q instead of %q", res, str)
		return
	}
}

func TestWriteFile(t *testing.T) {
	const str = "--- ZZZ ---"

	err := testAndCompare(str+" Ы", func(name string) (int64, error) {
		return WriteFile(name, 0644, String(str), Byte(' '), Rune('Ы'))
	})

	if err != nil {
		t.Error(err)
	}
}

func TestAtomicWriteFile(t *testing.T) {
	const str = "--- ZZZ ---"

	err := testAndCompare(str, func(name string) (int64, error) {
		return AtomicWriteFile(name, 0644, String(str))
	})

	if err != nil {
		t.Error(err)
	}
}

func TestAtomicWriteFileError(t *testing.T) {
	const file = "test-atomic-write-error"

	defer os.Remove(file)

	// write the file
	_, err := AtomicWriteFile(file, 0644, Repeat(func(i int, w *Writer) (int64, error) {
		if i < 5 {
			n, err := w.WriteString("ZZZ")
			return int64(n), err
		}

		return 0, errors.New("test error")
	}))

	// check if we've got an error
	if err == nil {
		t.Error("Missing error")
		return
	}

	// check error message
	const msg = "writing stream chunk 0: test error"

	if s := err.Error(); s != msg {
		t.Errorf("Unexpected error message: %q instead of %q", s, msg)
		return
	}

	// check for leftovers
	files, err := filepath.Glob("./tmp-*")

	if err != nil {
		t.Error(err)
		return
	}

	if len(files) > 0 {
		t.Error("Found unexpected temporary files:", join(", ", files...))
		return
	}

	// check if the target still exists (it should not)
	if _, err = os.Stat(file); !os.IsNotExist(err) {
		if err == nil {
			t.Errorf("File %q still exists", file)
			return
		}

		t.Errorf("Unexpected error while stat'ing file %q: %s", file, err)
		return
	}
}

func TestAtomicWriteFilePanic(t *testing.T) {
	const (
		file = "test-atomic-write-panic"
		cont = "ZZZ"
	)

	_, err := WriteFile(file, 0644, String(cont))

	if err != nil {
		t.Error(err)
		return
	}

	defer os.Remove(file)

	const pmsg = "this is panic"

	// panic handler
	defer func() {
		// check panic
		p := recover()

		if p == nil {
			t.Error("No panic :(")
			return
		}

		s, ok := p.(string)

		if !ok {
			t.Error("Unexpected panic:", p)
			return
		}

		if s != pmsg {
			t.Errorf("Unexpected panic message: %q instead of %q", s, pmsg)
			return
		}

		// check for leftovers
		files, err := filepath.Glob("./tmp-*")

		if err != nil {
			t.Error(err)
			return
		}

		if len(files) > 0 {
			t.Error("Found unexpected temporary files:", join(", ", files...))
			return
		}

		// check the target content
		content, err := ioutil.ReadFile(file)

		if err != nil {
			t.Error(err)
			return
		}

		if s := string(content); s != cont {
			t.Errorf("Unexpected file content: %q instead of %q", s, cont)
			return
		}
	}()

	// write the file
	_, err = AtomicWriteFile(file, 0644, Repeat(func(i int, w *Writer) (int64, error) {
		if i < 5 {
			n, err := w.WriteString("AAA")
			return int64(n), err
		}

		panic(pmsg)
	}))

	// check if we've got an error
	if err != nil {
		t.Error("Unexpected error:", err)
		return
	}
}

func TestAppendToFile(t *testing.T) {
	tmp, _, err := WriteTempFile(String("ZZZ"))

	if err != nil {
		t.Error(err)
		return
	}

	defer os.Remove(tmp)

	if _, err = AppendToFile(tmp, 0644, String("aaa")); err != nil {
		t.Error(err)
		return
	}

	cont, err := ioutil.ReadFile(tmp)

	if err != nil {
		t.Error(err)
		return
	}

	const exp = "ZZZaaa"

	if s := string(cont); s != exp {
		t.Errorf("Unexpected content: %q instead of %q", s, exp)
		return
	}
}

func TestWriteCloser(t *testing.T) {
	const str = "--- ZZZ ---"

	err := testAndCompareFd(str, func(file *os.File) (int64, error) {
		return WriteCloserStream(file).Write(String(str))
	})

	if err != nil {
		t.Error(err)
	}
}

func TestAll(t *testing.T) {
	err := testAndCompare("AAABBBCCC", func(name string) (int64, error) {
		return WriteFile(name, 0644, All(
			String("AAA"),
			String("BBB"),
			String("CCC"),
		))
	})

	if err != nil {
		t.Error(err)
	}
}

func TestFromFile(t *testing.T) {
	name, err := writeTempFile("ZZZ")

	if err != nil {
		t.Error(err)
		return
	}

	defer os.Remove(name)

	var b bytes.Buffer

	n, err := ByteBufferStream(&b).Write(
		String("--- "),
		File(name),
		String(" ---"),
	)

	if err != nil {
		t.Error(err)
		return
	}

	const str = "--- ZZZ ---"

	m := int64(len(str))

	if n != m {
		t.Errorf("Unexpected number of bytes written: %d instead of %d", n, m)
		return
	}

	if int64(b.Len()) != n {
		t.Errorf("Unexpected number of bytes in the buffer: %d instead of %d", b.Len(), n)
		return
	}

	if res := b.String(); res != str {
		t.Errorf("Unexpected result: %q instead of %q", res, str)
		return
	}
}

func TestDefaultFunctions(t *testing.T) {
	var w writer

	_, err := WriterStream(&w).Write(
		Rune('Ы'),
		Byte('z'),
		String("__"),
		ByteSlice([]byte("xxx")),
	)

	if err != nil {
		t.Error(err)
		return
	}

	const exp = "Ыz__xxx"

	if s := string(w.b); s != exp {
		t.Errorf("Unexpected result: %q instead of %q", s, exp)
		return
	}
}

func TestCommand(t *testing.T) {
	const cont = "ZZZ"

	var b bytes.Buffer

	if _, err := ByteBufferStream(&b).Write(Command("echo", cont)); err != nil {
		t.Error(err)
		return
	}

	if s := string(bytes.TrimSpace(b.Bytes())); s != cont {
		t.Errorf("Unexpected result: %q instead of %q", s, cont)
		return
	}
}

func TestCommadError(t *testing.T) {
	cmd := choose(runtime.GOOS == "windows", "type", "cat")

	var b bytes.Buffer

	_, err := ByteBufferStream(&b).Write(Command(cmd, "this-file-does-not-exist"))

	if err == nil {
		t.Error("No error returned")
		return
	}

	// "writing stream chunk 0: cat: this-file-does-not-exist: No such file or directory"
	t.Log(err)
}

func TestCommadStreamError(t *testing.T) {
	tmp, _, err := WriteTempFile(RepeatN(1000000, String("ZZZ\n")))

	if err != nil {
		t.Error(err)
		return
	}

	defer os.Remove(tmp)

	cmd := choose(runtime.GOOS == "windows", "type", "cat")

	var b deadWriter

	if _, err = WriterStream(&b).Write(Command(cmd, tmp)); err == nil {
		t.Error("missing error")
		return
	}

	const msg = "writing stream chunk 0: dead writer error"

	if s := err.Error(); s != msg {
		t.Errorf("Unexpected error message: %q instead of %q", s, msg)
		return
	}
}

type deadWriter struct{}

func (*deadWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("dead writer error")
}

// examples ------------------------------------------------------------------
func Example_hello() {
	_, err := WriterBufferedStream(os.Stdout).Write(
		String("Hello"),
		Byte(','),
		Rune(' '),
		String("world"),
		RepeatN(3, Byte('!')),
	)

	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// Hello, world!!!
}

func Example_file() {
	// write temporary file
	tmp, _, err := WriteTempFile(String("Hello, world!"))

	if err != nil {
		log.Fatal(err)
	}

	defer os.Remove(tmp)

	// 'cat' the temporary file
	_, err = WriterStream(os.Stdout).Write(File(tmp))

	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// Hello, world!
}

// helper functions -----------------------------------------------------------
func testAndCompare(expected string, test func(string) (int64, error)) error {
	name, err := mktemp("zzz-")

	if err != nil {
		return err
	}

	defer os.Remove(name)

	n, err := test(name)

	if err != nil {
		return err
	}

	return checkContent(name, expected, n)
}

func testAndCompareFd(expected string, test func(*os.File) (int64, error)) error {
	fd, err := ioutil.TempFile("", "zzz-")

	if err != nil {
		return err
	}

	name := fd.Name()

	defer func() {
		fd.Close()
		os.Remove(name)
	}()

	n, err := test(fd)

	if err != nil {
		return err
	}

	return checkContent(name, expected, n)
}

func checkContent(fileName, content string, n int64) error {
	if m := int64(len(content)); n != m {
		return fmt.Errorf("Unexpected number of bytes written: %d instead of %d", n, m)
	}

	res, err := ioutil.ReadFile(fileName)

	if err != nil {
		return err
	}

	if s := string(res); s != content {
		return fmt.Errorf("Unexpected result: %q instead of %q", s, content)
	}

	return nil
}

func writeTempFile(content string) (name string, err error) {
	name, _, err = WriteTempFile(String(content))

	return
}

func mktemp(prefix string) (string, error) {
	file, err := ioutil.TempFile("", prefix)

	if err != nil {
		return "", err
	}

	defer file.Close()

	return file.Name(), nil
}

func join(sep string, s ...string) string {
	return strings.Join(s, sep)
}

func choose(cond bool, alt1, alt2 string) string {
	if cond {
		return alt1
	}

	return alt2
}

// dummy writer
type writer struct {
	b []byte
}

func (w *writer) Write(s []byte) (int, error) {
	w.b = append(w.b, s...)
	return len(s), nil
}
