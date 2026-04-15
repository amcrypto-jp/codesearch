// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || freebsd || openbsd || netbsd
// +build darwin freebsd openbsd netbsd

package index

import (
	"log"
	"os"
	"syscall"
)

// missing from package syscall on freebsd, openbsd
const (
	_PROT_READ  = 1
	_MAP_SHARED = 1
)

func mmapFile(f *os.File) mmapData {
	st, err := f.Stat()
	if err != nil {
		log.Fatal(err)
	}
	size := st.Size()
	if int64(int(size+4095)) != size+4095 {
		log.Fatalf("%s: too large for mmap", f.Name())
	}
	n := int(size)
	if n == 0 {
		return mmapData{f: f}
	}
	data, err := syscall.Mmap(int(f.Fd()), 0, (n+4095)&^4095, _PROT_READ, _MAP_SHARED)
	if err != nil {
		log.Fatalf("mmap %s: %v", f.Name(), err)
	}
	return mmapData{f: f, d: data[:n], raw: data}
}

func unmmapFile(mm *mmapData) error {
	var err error
	if len(mm.raw) > 0 {
		err = syscall.Munmap(mm.raw)
	}
	if mm.f != nil {
		if e := mm.f.Close(); err == nil {
			err = e
		}
	}
	*mm = mmapData{}
	return err
}
