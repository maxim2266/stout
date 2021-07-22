# stout

[![GoDoc](https://godoc.org/github.com/maxim2266/stout?status.svg)](https://godoc.org/github.com/maxim2266/stout)
[![Go report](http://goreportcard.com/badge/maxim2266/stout)](http://goreportcard.com/report/maxim2266/stout)
[![License: BSD 3 Clause](https://img.shields.io/badge/License-BSD_3--Clause-yellow.svg)](https://opensource.org/licenses/BSD-3-Clause)

Package `stout` (STream OUTput): writing byte streams in a type-safe and extensible way.

### Motivation
In Go, writing files or other byte streams (pipes, sockets, etc.) is where all that error-checking
boilerplate code pops up, obscuring program logic and making the source code harder to read.
This project is an attempt to improve the situation. The main goals of the project are:
* Reduce the amount of error-checking boilerplate code;
* Make writing multi-part byte streams look more like a declarative composition, as much as possible with Go syntax;
* Develop an API that can be easily extended for any custom type or operation;
* Provide some generally useful functions for real-life applications.

The core ideas behind the implementation of the package are described in [this](https://maxim2266.github.io/stout/) blog post.

### Examples

##### "Hello, user" application:
```Go
func main() {
    _, err := stout.WriterBufferedStream(os.Stdout).Write(
        stout.String("Hello, "),
        stout.String(os.Getenv("USER")),
        stout.String("!\n"),
    )

    if err != nil {
        println("ERROR:", err.Error())
        os.Exit(1)
    }
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

##### Report generator
A complete application that generates a simple HTML page from `ps` command output:
```go
func main() {
    // run "ps" command
    lines, err := ps()

    if err != nil {
        die(err.Error())
    }

    // compose HTML
    // note: in a real-life application we would be writing to a socket instead of stdout
    _, err = stout.WriterBufferedStream(os.Stdout).Write(
        stout.String(hdr),
        stout.Repeat(rowsFrom(lines)),
        stout.String("</table></body></html>"),
    )

    if err != nil {
        die(err.Error())
    }
}

func rowsFrom(lines [][]byte) func(int, *stout.Writer) (int64, error) {
    return func(i int, w *stout.Writer) (int64, error) {
        // iteration stop
        if i >= len(lines) {
            return 0, io.EOF
        }

        // get the i-th record
        fields := bytes.Fields(lines[i])

        if len(fields) < 4 {
            return 0, io.EOF
        }

        // compose and write one row
        return w.WriteChunks([]stout.Chunk{
            stout.String("<tr><td>"),
            stout.Join("</td><td>",
                stout.ByteSlice(fields[0]),                         // pid
                stout.ByteSlice(fields[1]),                         // %cpu
                stout.ByteSlice(fields[2]),                         // %mem
                stout.String(html.EscapeString(string(fields[3]))), // cmd
            ),
            stout.String("</td></tr>"),
        })
    }
}

func ps() (lines [][]byte, err error) {
    var buff bytes.Buffer

    // get the output of the command into the buffer
    _, err = stout.ByteBufferStream(&buff).Write(
        stout.Command("ps", "--no-headers", "-Ao", "pid,%cpu,%mem,cmd"),
    )

    if err == nil {
        lines = bytes.Split(buff.Bytes(), []byte{'\n'})
    }

    return
}

func die(msg string) {
    println("error:", msg)
    os.Exit(1)
}

const hdr = `<!DOCTYPE html>
<html><head><meta charset="UTF-8"></head>
<body><table>
<tr><th>pid</th><th>cpu</th><th>mem</th><th>cmd</th>`
```

### Status
Tested on Linux Mint 20.2 with Go version 1.16.6.
