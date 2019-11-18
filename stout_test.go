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
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestBasics(t *testing.T) {
	const str = "Hello, world!"

	var s strings.Builder

	n, err := StringBuilderStream(&s).Write(
		String("Hello"),
		Rune(','),
		Byte(' '),
		ByteSlice([]byte("world!")),
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
	name, err := tempFileName("zzz-")

	if err != nil {
		t.Error(err)
		return
	}

	defer os.Remove(name)

	const str = "--- ZZZ ---"

	err = testFileWrite(name, str+" Ы", func() (int64, error) {
		return WriteFile(name, 0644, String(str), Byte(' '), Rune('Ы'))
	})

	if err != nil {
		t.Error(err)
	}
}

func TestAtomicWriteFile(t *testing.T) {
	name, err := tempFileName("zzz-")

	if err != nil {
		t.Error(err)
		return
	}

	defer os.Remove(name)

	const str = "--- ZZZ ---"

	err = testFileWrite(name, str, func() (int64, error) {
		return AtomicWriteFile(name, 0644, String(str))
	})

	if err != nil {
		t.Error(err)
	}
}

func TestWriteCloser(t *testing.T) {
	file, err := ioutil.TempFile("", "zzz-")

	if err != nil {
		t.Error(err)
		return
	}

	defer os.Remove(file.Name())

	const str = "--- ZZZ ---"

	err = testFileWrite(file.Name(), str, func() (int64, error) {
		return WriteCloserStream(file).Write(String(str))
	})

	if err != nil {
		t.Error(err)
	}
}

func testFileWrite(file, content string, test func() (int64, error)) error {
	n, err := test()

	if err != nil {
		return err
	}

	m := int64(len(content))

	if n != m {
		return fmt.Errorf("Unexpected number of bytes written: %d instead of %d", n, m)
	}

	res, err := ioutil.ReadFile(file)

	if err != nil {
		return err
	}

	if string(res) != content {
		return fmt.Errorf("Unexpected result: %q instead of %q", string(res), content)
	}

	return nil
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

func writeTempFile(content string) (name string, err error) {
	var file *os.File

	if file, err = ioutil.TempFile("", "zzz-"); err != nil {
		return
	}

	defer func() {
		if e := file.Close(); e != nil && err == nil {
			err = e
		}
	}()

	name = file.Name()
	_, err = file.WriteString(content)
	return
}

func tempFileName(prefix string) (string, error) {
	file, err := ioutil.TempFile("", prefix)

	if err != nil {
		return "", err
	}

	defer file.Close()

	return file.Name(), nil
}
