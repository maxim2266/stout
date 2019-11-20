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
	tmp, err := mktemp("zzz-")

	if err != nil {
		t.Error(err)
		return
	}

	defer os.Remove(tmp)

	_, err = AtomicWriteFile(tmp, 0644, Repeat(func(i int, w *Writer) (int64, error) {
		if i < 5 {
			n, err := w.WriteString("ZZZ")
			return int64(n), err
		}

		return 0, errors.New("test error")
	}))

	if err == nil {
		t.Error("Missing error")
	}

	const msg = "writing stream chunk 0: test error"

	if s := err.Error(); s != msg {
		t.Errorf("Unexpected error message: %q instead of %q", s, msg)
		return
	}

	files, err := filepath.Glob("./zzz-*")

	if err != nil {
		t.Error(err)
		return
	}

	if len(files) > 0 {
		t.Error("Found unexpected temporary files: " + join(", ", files...))
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

// dummy writer
type writer struct {
	b []byte
}

func (w *writer) Write(s []byte) (int, error) {
	w.b = append(w.b, s...)
	return len(s), nil
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
	tmp, _, err := WriteTempFile(String("Hello, world!"))

	if err != nil {
		log.Fatal(err)
	}

	defer os.Remove(tmp)

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
