// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/google/codesearch/regexp"
)

var usageMessage = `usage: cgrep [-c] [-h] [-i] [-l [-0]] [-n] [-v] regexp [file...]

cgrep behaves like grep, searching for regexp, an RE2 regular expression.

Options:

  -c           print only a count of selected lines
  -h           suppress the file name prefix on output
  -i           case-insensitive search
  -l           print only the names of files containing matches
  -0           with -l, print NUL-separated file names
  -n           print line numbers
  -v           print lines that do not match
  -cpuprofile FILE
               write CPU profile to FILE

As per Go's flag parsing convention, options cannot be combined: -i -n cannot
be abbreviated to -in.
`

func usage() {
	fmt.Fprintf(os.Stderr, usageMessage)
	os.Exit(2)
}

var (
	iflag      = flag.Bool("i", false, "case-insensitive match")
	cpuProfile = flag.String("cpuprofile", "", "write cpu profile to this file")
)

func main() {
	log.SetPrefix("cgrep: ")
	var g regexp.Grep
	g.AddFlags()
	g.AddVFlag()
	g.Stdout = os.Stdout
	g.Stderr = os.Stderr
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 || g.Z && !g.L {
		flag.Usage()
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	pat := "(?m)" + args[0]
	if *iflag {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		log.Fatal(err)
	}
	g.Regexp = re
	if len(args) == 1 {
		g.Reader(os.Stdin, "<standard input>")
	} else {
		for _, arg := range args[1:] {
			g.File(arg)
		}
	}
	if !g.Match {
		os.Exit(1)
	}
}
