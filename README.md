# headlines

[![GoDoc](https://godoc.org/github.com/e-dard/headlines?status.svg)](https://godoc.org/github.com/e-dard/headlines)

Markov Chain Generator focussed on generating headlines or single sentences.

## What?

[Markov Chains](http://en.wikipedia.org/wiki/Markov_chain).

This package generates a Markov Chain from a data stream of line-separated phrases.

## Usage

A minimal implementation follows.
In this case we're generating 100 phrases, each of which being a maximum of 20 words long. When generating each phrase the previous two
words are used to select the next word in the phrase.

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/e-dard/dailymarkov/generator"
)

func main() {
	if fi, err = os.Open("/tmp/foo.txt"); err != nil {
		log.Fatal(err)
	}
	defer fi.Close()

	chain := generator.NewChain(2)
	if err := chain.Build(fi); err != nil {
		log.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		text, err := chain.Generate(20)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(text)
	}
}
```
