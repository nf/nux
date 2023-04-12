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
	mem []byte // view of main memory

	append   bool
	nameAddr uint16
	name     string
	length   uint16
	success  uint16

	reader io.ReadCloser
	writer io.WriteCloser
}

func (f *File) In(d byte) byte {
	panic("not implemented")
}

func (f *File) InShort(d byte) uint16 {
	switch d & 0x0f {
	case 0x02:
		return f.success
	case 0x08:
		f.close()
		return f.nameAddr
	case 0x0a:
		return f.length
	default:
		panic("not implemented")
	}
}

func (f *File) Out(d, b byte) {
	switch d & 0x0f {
	case 0x07:
		f.append = d == 0x01
	default:
		panic("not implemented")
	}
}

func (f *File) OutShort(d byte, b uint16) {
	switch d & 0x0f {
	case 0x04: // stat
		f.success = 0
		if f.length == 0 {
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
		var buf bytes.Buffer
		writeFileInfo(&buf, fi)
		if buf.Len() > int(f.length) {
			return
		}
		f.success = uint16(copy(f.mem[b:b+f.length], buf.Bytes()))

	case 0x06: // delete
		if f.name == "" {
			panic("file delete before setting name")
		}
		if err := os.Remove(f.name); err != nil {
			log.Printf("delete file: %v", err)
			return
		}

	case 0x08: // name
		f.close()
		name, _, ok := bytes.Cut(f.mem[b:], []byte{0})
		if !ok {
			panic("unterminated file name")
		}
		n := path.Clean(string(name))
		if path.IsAbs(n) || strings.HasPrefix(n, "../") {
			panic(fmt.Errorf("bad file name %q", n))
		}
		f.nameAddr = b
		f.name = n

	case 0x0a: // length
		f.length = b

	case 0x0c: // read
		f.success = 0
		if f.length == 0 {
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
		n, err := f.reader.Read(f.mem[b : b+f.length])
		if err != nil && err != io.EOF {
			log.Printf("reading file: %v", err)
			return
		}
		f.success = uint16(n)

	case 0x0e: // write
		f.success = 0
		if f.length == 0 {
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
		n, err := f.writer.Write(f.mem[b : b+f.length])
		if err != nil {
			log.Printf("writing file: %v", err)
			return
		}
		f.success = uint16(n)

	default:
		panic("not implemented")
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
		var buf bytes.Buffer
		for _, fi := range fis {
			buf.Reset()
			writeFileInfo(&buf, fi)
			w.Write(buf.Bytes())
		}
	}()
	return io.NopCloser(r), nil
}

func writeFileInfo(buf *bytes.Buffer, fi fs.FileInfo) {
	if fi.IsDir() {
		buf.WriteString("---- ")
	} else if size := fi.Size(); size > 0xffff {
		buf.WriteString("???? ")
	} else {
		fmt.Fprintf(buf, "%.4x ", size)
	}
	buf.WriteString(filepath.ToSlash(fi.Name()))
	buf.WriteByte('\n')
}
