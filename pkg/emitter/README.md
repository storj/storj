emitter
--------

A simple event emitter for Go.

## Usage

If you've ever used an event emitter before, using Emission should be very familiar.

```go
package main

import (
  "fmt"
)

import (
  "storj.io/storj/pkg/emitter"
)

func main() {
  emitter := emitter.NewEmitter()

  hello := func(to string) {
    fmt.Printf("Hello %s!\n", to)
  }
  
  count := func(count int) {
    for i := 0; i < count; i++ {
      fmt.Println(i)
    }
  }
  
  emitter.On("hello", hello).
    On("count", count).
    Emit("hello", "world").
    Emit("count", 5)
}

```

## About

The `emission` package provides an event emitter making use of the reflect packages ability to call functions.  Using the `Call` method on the value of a function allows passing any type of function to the event emiiter, regardless of the functions parameters.

## Documentation

View godoc's or visit [godoc.org](http://godoc.org/github.com/chuckpreslar/emission).

    $ godoc emission

## License

Credit goes to Chuck Preslar [github.com/chuckpreslar]() for creating this library. 

> The MIT License (MIT)

> Copyright (c) 2013 - 2015 Chuck Preslar

> Permission is hereby granted, free of charge, to any person obtaining a copy
> of this software and associated documentation files (the "Software"), to deal
> in the Software without restriction, including without limitation the rights
> to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
> copies of the Software, and to permit persons to whom the Software is
> furnished to do so, subject to the following conditions:

> The above copyright notice and this permission notice shall be included in
> all copies or substantial portions of the Software.

> THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
> IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
> FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
> AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
> LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
> OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
> THE SOFTWARE.
