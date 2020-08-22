# Function composition and higher-order functions in Go

This post outlines the core ideas behind the `stout` package by developing a (much simplified)
library of composable byte streams.

### Background

Comparing Go to other programming languages, people often mention that
* Go offers no "generics", and
* Go requires explicit error checks that result in boilerplate code like `if err != nil { ... }`.

The only form of generic programming offered by Go is via `go generate` command, which is very powerful, but
requires quite some set-up, and many people find it to be just not worth the effort for simple things like
generic linked lists. Other often employed substitutes for generics are:
* "little copying", followed by edits: this quickly runs out of control if "little" is defined to be
greater than "once" or "twice";
* `interface{}`: works, but without type safety and with the overhead of reflection. Also, for a given
function it only works for a hard-coded set of real types.

But sometimes we can come up with a sufficiently generic solution without truly "generic" programming.
This can be achieved by looking at a bigger picture, i.e. by considering not just a generic container of
values of some type `T`, but also thinking about what we are going to do with that container. Below there is
an example of how this idea can work in the particular case of writing byte streams (files, sockets,
etc.).

### Composable Byte Streams

Doing i/o in Go is where all those explicit error checks become really annoying. Every operation
may fail, and we have to check for the errors:
```go
func writeFile(w io.Writer) error {
	if err := writeHeader(w); err != nil {
		return err
	}

	if err := writeBody(w); err != nil {
		return err
	}

	if err := writeFooter(w); err != nil {
		return err
	}

	return nil
}
```
And as the structure of the file we are writing becomes more complex, the code gets really
difficult to read and maintain. To improve on that, we would probably want to encapsulate most
of the writing machinery in a package so that we don't have to repeat the code over and over again.
Let's call the package `sw` (like "Stream Writer").
At the core of the package we want to see a function `Write()` with the following signature:
```go
func Write(w io.Writer, chunks ...Chunk) error
```
The function takes the destination byte stream in the form of `io.Writer`, and writes all the given
parts ("chunks"), from left to right, returning an error, or `nil`. With this function, the above
example can be rewritten like:
```go
if err := sw.Write(w, header, body, footer); err != nil {
	return err
}
```
At this point, the big question is: what is that `Chunk`? Designing the `sw` package,
we have no advance knowledge of what that type may be, so we have to use something generic instead.
The often suggested and used solutions are:

* `Chunk` is a `string`. Almost everything can be represented as a string, so it may be a reasonable
approach in simple cases, but if (just for example) the chunk is actually a disk file of 1Gb in size
then we have to read the entire file into the memory as one string, also checking the error from that
operation separately. This is rather sub-optimal.
* `Chunk` is `interface{}`, in the same manner as in `fmt.Printf` and similar functions. Here we can get
the convenience of writing integral types without explicit conversion, like
`sw.Write(w, "zzz", 123, 3.1415926)`, provided that the function knows how to serialise those
types to an `io.Writer`. Obviously, we will have to give up all the benefits of type safety,
and use reflection internally. Also, this does not scale up to dealing with any yet unknown type,
unless we require that type to implement `fmt.Stringer` (or similar) interface, which may or may
not be easy to achieve.

I think the most confusing bit here is that the word `chunk` is a noun, so we tend to think of that
chunk as some kind of a container for some data that we can write to a byte stream, but in fact what we
really want is to _write_ something, not to contain. Here `write` is a verb, and in software we (usually)
implement verbs as functions. Thinking this way, we can say that
```go
type Chunk = func(io.Writer) error
```
No interfaces, no "generics", just a function that takes a byte stream and returns an error, if any.

Given that, our implementation of the `Write()` function becomes really simple:
```go
func Write(w io.Writer, chunks ...Chunk) (err error) {
	for _, chunk := range chunks {
		if err = chunk(w); err != nil {
			break
		}
	}

	return
}
```

Now, having developed the core function of our package, we want to come up with a way to
map simple types to the chunk functions. The most suitable approach will be to use
[higher-order functions](https://en.wikipedia.org/wiki/Higher-order_function) that can take
a value of some data type and return a chunk function capable of writing that value
to a byte stream. We can start from these two simple functions:
```go
func Bytes(s []byte) Chunk {
	return func(w io.Writer) (err error) {
		_, err = w.Write(s)
		return
	}
}

func String(s string) Chunk {
	return Bytes([]byte(s))
}
```
These two small functions already allow us to say "hello" to the world:
```go
err := sw.Write(os.Stdout, sw.String("Hello, world!"))

if err != nil {
	// handle error
}
```
In fact, we can now produce some more meaningful output composed from a number of parts,
without all those boring error checks:
```go
err := sw.Write(os.Stdout,
	sw.String("This output is produced\n"),
	sw.String("using a composition of functions\n"),
	sw.String("each writing its own chunk of text.\n"),
)

if err != nil {
	// handle error
}
```
With little effort we can also develop other chunk function constructors for integers, floats, etc.
The implementations of those functions are trivial, so here we focus on more interesting code examples
instead, just to demonstrate what can be achieved with the library of three functions we have just
developed.

### Composing HTML

Let's say we want to write a file in HTML, a structured text format.

**Disclaimer:** _This is not going to be the nicest example of generating HTML in Go,
but rather an illustration of function composition methods._

First of all, in HTML we have to escape certain symbols, and for that we will need the
following function:
```go
func text(s string) sw.Chunk {
	return sw.String(html.EscapeString(s))
}
```
Using this function we can develop a constructor for a function that outputs the given text in
**bold** (i.e. between `<b>` and `</b>`):
```go
func b(s string) sw.Chunk {
	return func(w io.Writer) error {
		return sw.Write(w,
			sw.String("<b>"),
			text(s),
			sw.String("</b>"),
		)
	}
}
```
This works, but
* we have to repeat this code for each tag type, and
* there may be more structure between opening and closing tags than just a string of text.

Here we would want something more generic that could encapsulate the most of the boilerplate code.
We start from the following function that constructs a chunk function that writes the given list of
chunks enclosed in the given tag:
```go
func tag(t string, chunks ...sw.Chunk) sw.Chunk {
	list := make([]sw.Chunk, 0, len(chunks)+2)

	list = append(list, sw.String("<"+t+">"))
	list = append(list, chunks...)
	list = append(list, sw.String("</"+t+">"))

	return func(w io.Writer) error {
		return sw.Write(w, list...)
	}
}
```
And many other HTML chunk constructors become just a specialisation of the above generic function.
But we don't want to write all those specialisations ourselves, instead we automate the process with
another higher-order function:
```go
func textInTag(t string) func(string) sw.Chunk {
	return func(s string) sw.Chunk {
		return tag(t, text(s))
	}
}
```
Just to clarify, `textInTag` is a function that for a given tag name returns a function that for
a given string of text returns a chunk function that writes that text as HTML enclosed in
the tag. (Yes, this sentence sounds a bit scary.)

Now we can create chunk constructors on the fly:
```go
title := textInTag("title")
h3 := textInTag("h3")
// etc.
```
We also want to have a similar function `chunksInTag()` for wrapping a list of chunks instead of
simple text, but it's implementation is trivial.

Given the above set-up, we can finally write our HTML:
```go
// chunk constructors named after their corresponding HTML tags
// (could even be created statically)
b := textInTag("b")
i := textInTag("i")
title := textInTag("title")
h3 := textInTag("h3")
body := chunksInTag("body")
html_ := chunksInTag("html") // underscore to avoid name collision with `html` package
head := chunksInTag("head")
footer := chunksInTag("footer")

// compose html
err := sw.Write(os.Stdout,
	sw.String("<!DOCTYPE HTML>"),
	html_(
		head(
			sw.String(`<meta charset="utf-8">`),
			title("Function Composition"),
		),
		body(
			h3("Function Composition"),
			text("In computer science, "), b("function composition"),
			text(" is an act or mechanism to "), i("combine"),
			text(" simple functions to build more complicated ones."),
			footer(
				text("Definition from Wikipedia."),
			),
		),
	),
)

if err != nil {
	// handle error
}
```

### Conclusion

The above example demonstrates that:
* In situations where we know exactly the single operation we are going to apply to a
"generic" type, the type itself can be represented by a function that performs the operation.
Importantly, the signature of such a function has no reference to that generic type.
* The core of the i/o and error-checking boilerplate can be encapsulated in a package
as small as just three functions, each consisting of only a few lines of code. Nowhere else in this example
we had to explicitly check for errors, except the main function where we did the actual writing.
* The small package we developed here is extensible enough to allow for building a structured text
writer on top of it, with the help of just a few more functions.

