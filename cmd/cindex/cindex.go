// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime/pprof"
	"slices"
	"strings"

	"github.com/google/codesearch/index"
)

var usageMessage = `usage: cindex [-list] [-reset] [-zip] [options] [path...]

Cindex prepares the trigram index for use by csearch.  The index is the
file named by $CSEARCHINDEX, or else $HOME/.csearchindex. The -indexpath
flag uses a specific index file instead.

The simplest invocation is

	cindex path...

which adds the file or directory tree named by each path to the index.
For example:

	cindex $HOME/src /usr/include

or, equivalently:

	cindex $HOME/src
	cindex /usr/include

If cindex is invoked with no paths, it reindexes the paths that have
already been added, in case the files have changed.  Thus, 'cindex' by
itself is a useful command to run in a nightly cron job.

The -list flag causes cindex to list the paths it has indexed and exit.

The -zip flag causes cindex to index content inside ZIP files.
This feature is experimental and will almost certainly change
in the future, possibly in incompatible ways.

The -exclude flag names a file containing filepath patterns, one per line,
to exclude from indexing. Blank lines and lines beginning with # are ignored.

By default cindex skips hidden dot-files and dot-directories. The -includehidden
flag indexes hidden source files while still skipping VCS directories, backup
names, and explicit exclusions.

The -filelist flag names a file containing paths to index, one per line.

By default cindex adds the named paths to the index but preserves
information about other paths that might already be indexed
(the ones printed by cindex -list).  The -reset flag causes cindex to
delete the existing index before indexing the new paths.
With no path arguments, cindex -reset removes the index.
`

func usage() {
	fmt.Fprintf(os.Stderr, usageMessage)
	os.Exit(2)
}

var (
	listFlag          = flag.Bool("list", false, "list indexed paths and exit")
	resetFlag         = flag.Bool("reset", false, "discard existing index")
	verboseFlag       = flag.Bool("verbose", false, "print extra information")
	cpuProfile        = flag.String("cpuprofile", "", "write cpu profile to this file")
	checkFlag         = flag.Bool("check", false, "check index is well-formatted")
	indexPath         = flag.String("indexpath", "", "use this index file instead of $CSEARCHINDEX or $HOME/.csearchindex")
	logSkipFlag       = flag.Bool("logskip", false, "log information about skipped files")
	excludeFlag       = flag.String("exclude", "", "read file exclusion patterns from this file")
	includeHiddenFlag = flag.Bool("includehidden", false, "index hidden files and directories except VCS directories")
	fileList          = flag.String("filelist", "", "read paths to index from this file")
	maxFileLen        = flag.Int64("maxfilelen", index.DefaultMaxFileLen, "skip files longer than this many bytes")
	maxLineLen        = flag.Int("maxlinelen", index.DefaultMaxLineLen, "skip files with a line longer than this many bytes")
	maxTrigrams       = flag.Int("maxtrigrams", index.DefaultMaxTextTrigrams, "skip files with more than this many distinct trigrams")
	zipFlag           = flag.Bool("zip", false, "index content in zip files")
	statsFlag         = flag.Bool("stats", false, "print index size statistics")

	excludePatterns = []string{".csearchindex"}
)

func main() {
	log.SetPrefix("cindex: ")
	flag.Usage = usage
	flag.Parse()

	if *indexPath != "" {
		if err := os.Setenv("CSEARCHINDEX", expandTilde(*indexPath)); err != nil {
			log.Fatal(err)
		}
	}

	if *listFlag {
		ix := index.Open(index.File())
		defer ix.Close()
		if *checkFlag {
			if err := ix.Check(); err != nil {
				log.Fatal(err)
			}
		}
		for p := range ix.Roots().All() {
			fmt.Printf("%s\n", p)
		}
		return
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

	args := flag.Args()
	if *fileList != "" {
		args = append(args, readListFile(*fileList, "file list")...)
	}
	if *excludeFlag != "" {
		excludePatterns = append(excludePatterns, readListFile(*excludeFlag, "exclude patterns")...)
	}

	if *resetFlag && len(args) == 0 {
		os.Remove(index.File())
		return
	}
	var roots []index.Path
	if len(args) == 0 {
		ix := index.Open(index.File())
		roots = slices.Collect(ix.Roots().All())
		if err := ix.Close(); err != nil {
			log.Fatal(err)
		}
	} else {
		// Translate arguments to absolute paths so that
		// we can generate the file list in sorted order.
		for _, arg := range args {
			a, err := filepath.Abs(arg)
			if err != nil {
				log.Printf("%s: %s", arg, err)
				continue
			}
			roots = append(roots, index.MakePath(a))
		}
		slices.SortFunc(roots, index.Path.Compare)
	}

	master := index.File()
	if _, err := os.Stat(master); err != nil {
		// Does not exist.
		*resetFlag = true
	}
	file := master
	if !*resetFlag {
		file += "~"
		if *checkFlag {
			ix := index.Open(master)
			if err := ix.Check(); err != nil {
				log.Fatal(err)
			}
			if err := ix.Close(); err != nil {
				log.Fatal(err)
			}
		}
	}

	ix := index.Create(file)
	ix.Verbose = *verboseFlag
	ix.LogSkip = *verboseFlag || *logSkipFlag
	ix.Zip = *zipFlag
	ix.MaxFileLen = *maxFileLen
	ix.MaxLineLen = *maxLineLen
	ix.MaxTextTrigrams = *maxTrigrams
	ix.AddRoots(roots)
	for _, root := range roots {
		log.Printf("index %s", root)
		filepath.Walk(root.String(), func(path string, info os.FileInfo, err error) error {
			if _, elem := filepath.Split(path); elem != "" {
				if isExcluded(path, elem) {
					if ix.LogSkip {
						log.Printf("%s: excluded, ignoring", path)
					}
					if info != nil && info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if shouldSkipName(elem, info != nil && info.IsDir()) {
					if ix.LogSkip {
						log.Printf("%s: skipped, ignoring", path)
					}
					if info != nil && info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if err != nil {
				log.Printf("%s: %s", path, err)
				return nil
			}
			if info != nil && info.Mode()&os.ModeType == 0 {
				if err := ix.AddFile(path); err != nil {
					log.Printf("%s: %s", path, err)
					return nil
				}
			}
			return nil
		})
	}
	log.Printf("flush index")
	ix.Flush()

	if !*resetFlag {
		log.Printf("merge %s %s", master, file)
		index.Merge(file+"~", master, file)
		if *checkFlag {
			ix := index.Open(file + "~")
			if err := ix.Check(); err != nil {
				log.Fatal(err)
			}
			if err := ix.Close(); err != nil {
				log.Fatal(err)
			}
		}
		removeIndex(file)
		removeIndex(master)
		if err := os.Rename(file+"~", master); err != nil {
			log.Fatalf("failed to merge indexes: %v", err)
		}
	} else {
		if *checkFlag {
			ix := index.Open(file)
			if err := ix.Check(); err != nil {
				log.Fatal(err)
			}
			if err := ix.Close(); err != nil {
				log.Fatal(err)
			}
		}
	}

	log.Printf("done")

	if *statsFlag {
		ix := index.Open(master)
		defer ix.Close()
		ix.PrintStats()
	}
	return
}

func removeIndex(name string) {
	if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
		log.Fatalf("removing %s: %v", name, err)
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

func expandTilde(name string) string {
	if len(name) >= 2 && name[0] == '~' && (name[1] == '/' || name[1] == '\\') {
		return filepath.Join(index.HomeDir(), name[2:])
	}
	return name
}

func readListFile(name, kind string) []string {
	name = expandTilde(name)
	if *logSkipFlag {
		log.Printf("load %s from %s", kind, name)
	}
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
			if ok, err := pathMatch(pattern, slashFile); err != nil {
				log.Fatal(err)
			} else if ok {
				return true
			}
		}
	}
	return false
}

func pathMatch(pattern, name string) (bool, error) {
	pattern = strings.TrimPrefix(pattern, "./")
	name = strings.TrimPrefix(name, "./")
	return path.Match(pattern, name)
}
