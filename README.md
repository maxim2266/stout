# stout

[![GoDoc](https://godoc.org/github.com/maxim2266/stout?status.svg)](https://godoc.org/github.com/maxim2266/stout)
[![Go report](http://goreportcard.com/badge/maxim2266/stout)](http://goreportcard.com/report/maxim2266/stout)
[![License: BSD 3 Clause](https://img.shields.io/badge/License-BSD_3--Clause-yellow.svg)](https://opensource.org/licenses/BSD-3-Clause)

Package `stout` (STream OUTput): writing byte streams in a type-safe and extensible way.

### Examples

##### "Hello, user" application:
```Go
func main() {
	_, err := stout.WriterBufferedStream(os.Stdout).Write(
		stout.String("Hello, "),
		user(),
		stout.String("!\n"),
	)

	if err != nil {
		println("ERROR:", err.Error())
		os.Exit(1)
	}
}

func user() stout.Chunk {
	name := os.Getenv("USER")

	if len(name) == 0 {
		name = "<user>"
	}

	return stout.String(name)
}
```

##### Simplified implementation of `cat` command:
```Go
func main() {
	files := make([]stout.Chunk, 0, len(os.Args)-1)

	for _, arg := range os.Args[1:] {
		files = append(files, stout.File(arg))
	}

	_, err := stout.WriterBufferedStream(os.Stdout).Write(files...)

	if err != nil {
		println("ERROR:", err.Error())
		os.Exit(1)
	}
}
```
