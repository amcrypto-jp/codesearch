// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime/pprof"
	"strings"

	"github.com/google/codesearch/index"
	"github.com/google/codesearch/regexp"
)

var usageMessage = `usage: csearch [-c] [-f fileregexp] [-h] [-i] [-l [-0]] [-n] [options] regexp

Csearch behaves like grep over all indexed files, searching for regexp,
an RE2 (nearly PCRE) regular expression.

The -c, -h, -i, -l, -m, -M, and -n flags are as in grep, although note
that as per Go's flag parsing convention, they cannot be combined: the option pair -i -n
cannot be abbreviated to -in.

The -f flag restricts the search to files whose names match the RE2 regular
expression fileregexp.

The -all flag searches all regular files under the indexed roots after the
indexed search, so new or changed files not represented in the index are not
missed. The -exclude flag names a file containing filepath patterns, one per
line, to exclude during the -all walk. By default the -all walk skips hidden
dot-files and dot-directories. The -includehidden flag searches hidden files
while still skipping VCS directories, backup names, and explicit exclusions.

Csearch relies on the existence of an up-to-date index created ahead of time.
To build or rebuild the index that csearch uses, run:

	cindex path...

where path... is a list of directories or individual files to be included in the index.
If no index exists, this command creates one.  If an index already exists, cindex
overwrites it.  Run cindex -help for more.

Csearch uses the index stored in $CSEARCHINDEX or, if that variable is unset or
empty, $HOME/.csearchindex. The -indexpath flag uses a specific index file
instead.
`

func usage() {
	fmt.Fprintf(os.Stderr, usageMessage)
	os.Exit(2)
}

var (
	fFlag             = flag.String("f", "", "search only files with names matching this regexp")
	iFlag             = flag.Bool("i", false, "case-insensitive search")
	htmlFlag          = flag.Bool("html", false, "print HTML output")
	verboseFlag       = flag.Bool("verbose", false, "print extra information")
	bruteFlag         = flag.Bool("brute", false, "brute force - search all files in index")
	allFlag           = flag.Bool("all", false, "also search regular files under indexed roots")
	excludeFlag       = flag.String("exclude", "", "read -all file exclusion patterns from this file")
	includeHiddenFlag = flag.Bool("includehidden", false, "include hidden files and directories in -all search")
	cpuProfile        = flag.String("cpuprofile", "", "write cpu profile to this file")
	indexPath         = flag.String("indexpath", "", "use this index file instead of $CSEARCHINDEX or $HOME/.csearchindex")
	maxCount          = flag.Int("m", 0, "stop after this many matches")
	maxPerFile        = flag.Int("M", 0, "stop after this many matches in each file")

	matches bool

	excludePatterns = []string{".csearchindex"}
)

func Main() {
	log.SetPrefix("csearch: ")
	g := regexp.Grep{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	g.AddFlags()

	flag.Usage = usage
	flag.Parse()
	if *htmlFlag {
		g.HTML = true
	}
	args := flag.Args()

	if len(args) != 1 || g.Z && !g.L || g.Z && (g.C || g.H) || *maxPerFile > 0 && (g.C || g.L) || *maxCount > 0 && g.C {
		usage()
	}
	g.Limit = *maxCount
	g.FileLimit = *maxPerFile

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if *indexPath != "" {
		if err := os.Setenv("CSEARCHINDEX", *indexPath); err != nil {
			log.Fatal(err)
		}
	}

	pat := "(?m)" + args[0]
	if *iFlag {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		log.Fatal(err)
	}
	g.Regexp = re
	var fre *regexp.Regexp
	if *fFlag != "" {
		fre, err = regexp.Compile(*fFlag)
		if err != nil {
			log.Fatal(err)
		}
	}
	q := index.RegexpQuery(re.Syntax)
	if *verboseFlag {
		log.Printf("query: %s\n", q)
	}

	ix := index.Open(index.File())
	defer ix.Close()
	ix.Verbose = *verboseFlag
	var post []int
	if *bruteFlag {
		post = ix.PostingQuery(&index.Query{Op: index.QAll})
	} else {
		post = ix.PostingQuery(q)
	}
	if *verboseFlag {
		log.Printf("post query identified %d possible files\n", len(post))
	}

	if fre != nil {
		fnames := make([]int, 0, len(post))

		for _, fileid := range post {
			name := ix.Name(fileid)
			if fre.MatchString(name.String(), true, true) < 0 {
				continue
			}
			fnames = append(fnames, fileid)
		}

		if *verboseFlag {
			log.Printf("filename regexp matched %d files\n", len(fnames))
		}
		post = fnames
	}

	var (
		zipFile   string
		zipReader *zip.ReadCloser
		zipMap    map[string]*zip.File
	)

	seen := make(map[string]bool, len(post))
	for _, fileid := range post {
		name := ix.Name(fileid).String()
		seen[seenName(name)] = true
		if g.L && (pat == "(?m)" || pat == "(?i)(?m)") {
			g.Reader(bytes.NewReader(nil), name)
			if g.Done {
				break
			}
			continue
		}
		file, err := os.Open(string(name))
		if err != nil {
			if i := strings.Index(name, ".zip\x01"); i >= 0 {
				zfile, zname := name[:i+4], name[i+5:]
				if zfile != zipFile {
					if zipReader != nil {
						zipReader.Close()
						zipMap = nil
					}
					zipFile = zfile
					zipReader, err = zip.OpenReader(zfile)
					if err != nil {
						zipReader = nil
					}
					if zipReader != nil {
						zipMap = make(map[string]*zip.File)
						for _, file := range zipReader.File {
							zipMap[file.Name] = file
						}
					}
				}
				file := zipMap[zname]
				if file != nil {
					r, err := file.Open()
					if err != nil {
						continue
					}
					g.Reader(r, name)
					r.Close()
					if g.Done {
						break
					}
					continue
				}
			}
			continue
		}
		g.Reader(file, name)
		file.Close()
		if g.Done {
			break
		}
	}
	if *allFlag && !g.Done {
		if *excludeFlag != "" {
			excludePatterns = readListFile(*excludeFlag)
		}
		for root := range ix.Roots().All() {
			walkAll(root.String(), seen, fre, &g)
			if g.Done {
				break
			}
		}
	}

	matches = g.Match
}

func seenName(name string) string {
	if strings.Contains(name, "\x01") {
		return name
	}
	return filepath.Clean(name)
}

func walkAll(root string, seen map[string]bool, fre *regexp.Regexp, g *regexp.Grep) {
	err := filepath.Walk(root, func(file string, info os.FileInfo, err error) error {
		if g.Done {
			return filepath.SkipAll
		}
		if err != nil {
			if *verboseFlag {
				log.Printf("%s: %v", file, err)
			}
			return nil
		}
		if info == nil {
			return nil
		}
		if _, elem := filepath.Split(file); elem != "" {
			if isExcluded(file, elem) || shouldSkipName(elem, info.IsDir()) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if info.Mode()&os.ModeType != 0 {
			return nil
		}
		name := filepath.Clean(file)
		if seen[name] {
			return nil
		}
		seen[name] = true
		if fre != nil && fre.MatchString(name, true, true) < 0 {
			return nil
		}
		g.File(name)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

func shouldSkipName(elem string, isDir bool) bool {
	if elem == "" {
		return false
	}
	if elem[0] == '#' || elem[0] == '~' || elem[len(elem)-1] == '~' {
		return true
	}
	if isDir && isVCSDir(elem) {
		return true
	}
	return !*includeHiddenFlag && elem[0] == '.'
}

func isVCSDir(elem string) bool {
	switch elem {
	case ".git", ".hg", ".bzr", ".svn", ".svk", "SCCS", "CVS", "_darcs", "_MTN":
		return true
	}
	return false
}

func readListFile(name string) []string {
	name = expandTilde(name)
	data, err := os.ReadFile(name)
	if err != nil {
		log.Fatal(err)
	}
	var list []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		list = append(list, line)
	}
	return list
}

func expandTilde(name string) string {
	if len(name) >= 2 && name[0] == '~' && (name[1] == '/' || name[1] == '\\') {
		return filepath.Join(index.HomeDir(), name[2:])
	}
	return name
}

func isExcluded(file, elem string) bool {
	slashFile := filepath.ToSlash(file)
	for _, pattern := range excludePatterns {
		if ok, err := filepath.Match(pattern, elem); err != nil {
			log.Fatal(err)
		} else if ok {
			return true
		}
		if strings.Contains(pattern, "/") || strings.Contains(pattern, string(filepath.Separator)) {
			pattern = filepath.ToSlash(pattern)
			if ok, err := path.Match(strings.TrimPrefix(pattern, "./"), strings.TrimPrefix(slashFile, "./")); err != nil {
				log.Fatal(err)
			} else if ok {
				return true
			}
		}
	}
	return false
}

func main() {
	Main()
	if !matches {
		os.Exit(1)
	}
	os.Exit(0)
}
