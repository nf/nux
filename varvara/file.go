package varvara

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

func (f *File) data(addr uint16) []byte {
	end := min(int(addr)+int(f.length()), 0x10000)
	return f.main[int(addr):end]
}

func (f *File) In(p byte) byte {
	switch p {
	case 0x8, 0x9: // name
		f.close()
	}
	return f.mem[p]
}

func (f *File) Out(p, b byte) {
	f.mem[p] = b
	switch p {

	case 0x5: // stat
		f.setSuccess(0)
		fi, err := os.Stat(filepath.FromSlash(f.name))
		addr := f.mem.short(0x4)
		data := f.data(addr)
		n := copy(data, fileStatBytes(fi, err, len(data)))
		f.setSuccess(n)

	case 0x6: // delete
		f.setSuccess(0)
		if err := os.Remove(filepath.FromSlash(f.name)); err == nil {
			f.setSuccess(1)
		}

	case 0x7:
		f.append = b == 0x01

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
		if f.writer != nil {
			f.close()
		}
		if f.reader == nil {
			if f.name == "" {
				return
			}
			r, err := fileReader(filepath.FromSlash(f.name))
			if err != nil {
				return
			}
			f.reader = r
		}
		addr := f.mem.short(0xc)
		n, err := f.reader.Read(f.data(addr))
		if err != nil && err != io.EOF {
			return
		}
		f.setSuccess(n)

	case 0xf: // write
		f.setSuccess(0)
		if f.reader != nil {
			f.close()
		}
		if f.writer == nil {
			if f.name == "" {
				return
			}
			flag := os.O_WRONLY | os.O_CREATE
			if f.append {
				flag |= os.O_APPEND
			} else {
				flag |= os.O_TRUNC
			}
			fp, err := os.OpenFile(filepath.FromSlash(f.name), flag, 0644)
			if err != nil {
				return
			}
			f.writer = fp
		}
		addr := f.mem.short(0xe)
		n, err := f.writer.Write(f.data(addr))
		if err != nil {
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
	var buf bytes.Buffer
	for _, de := range des {
		fi, err := de.Info()
		if err != nil {
			return nil, err
		}
		buf.Write(fileInfoBytes(fi))
	}
	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

func fileStatBytes(fi fs.FileInfo, err error, length int) []byte {
	length = min(length, 4)
	if err != nil {
		return bytes.Repeat([]byte{'!'}, length)
	}
	if fi.IsDir() {
		return bytes.Repeat([]byte{'-'}, length)
	}
	if fi.Size() >= int64(1<<(length*4)) {
		return bytes.Repeat([]byte{'?'}, length)
	}
	return fmt.Appendf(nil, "%0*x", length, fi.Size())
}

func fileInfoBytes(fi fs.FileInfo) []byte {
	var buf bytes.Buffer
	if fi.IsDir() {
		buf.WriteString("----\t")
	} else if size := fi.Size(); size > 0xffff {
		buf.WriteString("????\t")
	} else {
		fmt.Fprintf(&buf, "%.4x\t", size)
	}
	buf.WriteString(filepath.ToSlash(fi.Name()))
	if fi.IsDir() {
		buf.WriteByte('/')
	}
	buf.WriteByte('\n')
	return buf.Bytes()
}
