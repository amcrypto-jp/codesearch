// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index

import (
	"log"
	"os"
	"reflect"
	"syscall"
	"unsafe"
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
	if size == 0 {
		return mmapData{f: f}
	}
	h, err := syscall.CreateFileMapping(syscall.Handle(f.Fd()), nil, syscall.PAGE_READONLY, uint32(size>>32), uint32(size), nil)
	if err != nil {
		log.Fatalf("CreateFileMapping %s: %v", f.Name(), err)
	}

	addr, err := syscall.MapViewOfFile(h, syscall.FILE_MAP_READ, 0, 0, 0)
	if err != nil {
		log.Fatalf("MapViewOfFile %s: %v", f.Name(), err)
	}
	var data []byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	hdr.Data = addr
	hdr.Len = int(size)
	hdr.Cap = int(size)
	return mmapData{f: f, d: data, raw: data, addr: addr, h: uintptr(h)}
}

func unmmapFile(mm *mmapData) error {
	var err error
	if mm.addr != 0 {
		err = syscall.UnmapViewOfFile(mm.addr)
	}
	if mm.h != 0 {
		if e := syscall.CloseHandle(syscall.Handle(mm.h)); err == nil {
			err = e
		}
	}
	if mm.f != nil {
		if e := mm.f.Close(); err == nil {
			err = e
		}
	}
	*mm = mmapData{}
	return err
}
