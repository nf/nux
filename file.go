package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type File struct {
	mem  deviceMem
	main []byte // view of main memory

	append bool
	name   string

	reader io.ReadCloser
	writer io.WriteCloser
}

func (f *File) setSuccess(v int) { f.mem.setShort(0x2, uint16(v)) }
func (f *File) length() uint16   { return f.mem.short(0xa) }

func (f *File) In(d byte) byte {
	switch d {
	case 0x8, 0x9: // name
		f.close()
	}
	return f.mem[d]
}

func (f *File) Out(d, b byte) {
	f.mem[d] = b
	switch d {

	case 0x5: // stat
		f.setSuccess(0)
		if f.length() == 0 {
			panic("file stat before setting length (or set zero length)")
		}
		if f.name == "" {
			panic("file stat before setting name")
		}
		fi, err := os.Stat(filepath.FromSlash(f.name))
		if err != nil {
			log.Printf("stat file: %v", err)
			return
		}
		info := fileInfoBytes(fi)
		if len(info) > int(f.length()) {
			return
		}
		addr := f.mem.short(0x4)
		n := copy(f.main[addr:addr+f.length()], info)
		f.setSuccess(n)

	case 0x6: // delete
		if f.name == "" {
			panic("file delete before setting name")
		}
		if err := os.Remove(filepath.FromSlash(f.name)); err != nil {
			log.Printf("delete file: %v", err)
		}

	case 0x7:
		f.append = d == 0x01

	case 0x9: // name
		f.close()
		addr := f.mem.short(0x8)
		name, _, ok := bytes.Cut(f.main[addr:], []byte{0})
		if !ok {
			panic("unterminated file name")
		}
		n := string(name)
		if n != "" {
			n = path.Clean(n)
			if path.IsAbs(n) || strings.HasPrefix(n, "../") {
				panic(fmt.Errorf("bad file name %q", n))
			}
		}
		f.name = n

	case 0xd: // read
		f.setSuccess(0)
		if f.length() == 0 {
			panic("file read before setting length (or set zero length)")
		}
		if f.writer != nil {
			panic("file read after write; should re-open")
		}
		if f.reader == nil {
			if f.name == "" {
				panic("file read before setting name")
			}
			r, err := fileReader(filepath.FromSlash(f.name))
			if err != nil {
				log.Printf("opening file: %v", err)
				return
			}
			f.reader = r
		}
		addr := f.mem.short(0xc)
		n, err := f.reader.Read(f.main[addr : addr+f.length()])
		if err != nil && err != io.EOF {
			log.Printf("reading file: %v", err)
			return
		}
		f.setSuccess(n)

	case 0xf: // write
		f.setSuccess(0)
		if f.length() == 0 {
			panic("file write before setting length (or set zero length)")
		}
		if f.reader != nil {
			panic("file write after read; should re-open")
		}
		if f.writer == nil {
			if f.name == "" {
				panic("file write before setting name")
			}
			flag := os.O_WRONLY | os.O_CREATE
			if f.append {
				flag |= os.O_APPEND
			}
			fp, err := os.OpenFile(filepath.FromSlash(f.name), flag, 0644)
			if err != nil {
				log.Printf("opening file: %v", err)
				return
			}
			f.writer = fp
		}
		addr := f.mem.short(0xe)
		n, err := f.writer.Write(f.main[addr : addr+f.length()])
		if err != nil {
			log.Printf("writing file: %v", err)
			return
		}
		f.setSuccess(n)
	}
}

func (f *File) close() {
	if w := f.writer; w != nil {
		if err := w.Close(); err != nil {
			log.Printf("closing file: %v", err)
		}
		f.writer = nil
	}
	if r := f.reader; r != nil {
		if err := r.Close(); err != nil {
			log.Printf("closing file: %v", err)
		}
		f.reader = nil
	}
}

func fileReader(name string) (io.ReadCloser, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return nil, err
	}

	if !fi.IsDir() {
		return os.Open(name)
	}

	des, err := os.ReadDir(name)
	if err != nil {
		return nil, err
	}
	var fis []fs.FileInfo
	for _, de := range des {
		fi, err := de.Info()
		if err != nil {
			return nil, err
		}
		fis = append(fis, fi)
	}
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		for _, fi := range fis {
			w.Write(fileInfoBytes(fi))
		}
	}()
	return io.NopCloser(r), nil
}

func fileInfoBytes(fi fs.FileInfo) []byte {
	var buf bytes.Buffer
	if fi.IsDir() {
		buf.WriteString("---- ")
	} else if size := fi.Size(); size > 0xffff {
		buf.WriteString("???? ")
	} else {
		fmt.Fprintf(&buf, "%.4x ", size)
	}
	buf.WriteString(filepath.ToSlash(fi.Name()))
	buf.WriteByte('\n')
	return buf.Bytes()
}
