package cab

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"time"
)

// OpenReader will open the Cab file specified by name and return a ReadCloser.
func OpenReader(name string) (*ReadCloser, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	var r ReadCloser
	if err := r.init(f, fi.Size()); err != nil {
		f.Close()
		return nil, err
	}

	r.f = f
	return &r, nil
}

// ReadCloser is a closable cab file.
type ReadCloser struct {
	Reader

	f *os.File
}

// Close closes the Cab file, rendering it unusable for I/O.
func (rc *ReadCloser) Close() error {
	return rc.f.Close()
}

// NewReader makes a Reader reading from r, which is assumed to ahve the give size in bytes.
func NewReader(r io.ReaderAt, size int64) (*Reader, error) {
	if size < 0 {
		return nil, errors.New("zip: size cannot be negative")
	}

	var c Reader
	if err := c.init(r, size); err != nil {
		return nil, err
	}

	return &c, nil
}

// Reader is a readable cab file.
type Reader struct {
	Folders []*Folder
	PrevCab *Ref
	NextCab *Ref

	size         uint32
	minorVersion uint8
	majorVersion uint8
	setID        uint16
	setIdx       uint16

	r io.ReaderAt
}

func (c *Reader) init(r io.ReaderAt, size int64) error {
	c.r = r
	rs := io.NewSectionReader(r, 0, size)
	buf := bufio.NewReader(rs)
	b := readBuf{buf: buf}

	// signature
	if b.uint32() != 0x4643534d { // "MSCF" stored little-endian
		return errors.New("invalid file signature")
	}

	b.skip(4)
	c.size = b.uint32()
	b.skip(4)
	firstFileOffset := b.uint32()
	b.skip(4)
	c.minorVersion = b.uint8()
	c.majorVersion = b.uint8()
	numFolders := b.uint16()
	numFiles := b.uint16()
	flags := b.uint16()
	c.setID = b.uint16()
	c.setIdx = b.uint16()

	// reserves
	var cabinetReserveSize uint16
	var folderReserveSize uint8
	var dataReserveSize uint8
	if flags&0x4 != 0 {
		cabinetReserveSize = b.uint16()
		folderReserveSize = b.uint8()
		dataReserveSize = b.uint8()
	}

	b.skip(int(cabinetReserveSize))

	if flags&0x01 != 0 {
		c.PrevCab = &Ref{
			Name: b.nullTerminatedString(),
			Disk: b.nullTerminatedString(),
		}
	}

	if flags&0x02 != 0 {
		c.NextCab = &Ref{
			Name: b.nullTerminatedString(),
			Disk: b.nullTerminatedString(),
		}
	}

	if b.err != nil {
		return b.err
	}

	c.Folders = make([]*Folder, 0, numFolders)
	for i := 0; i < int(numFolders); i++ {
		c.Folders = append(c.Folders, &Folder{
			firstDataOffset: b.uint32(),
			numDataBlocks:   b.uint16(),
			compressionBits: b.uint16(),
			compressionType: b.uint8(),
		})

		b.skip(int(folderReserveSize))
	}

	if _, err := rs.Seek(int64(firstFileOffset), io.SeekStart); err != nil {
		return err
	}
	buf.Reset(rs)

	for i := 0; i < int(numFiles); i++ {
		file := &File{
			uncompressedSize:   b.uint32(),
			uncompressedOffset: b.uint32(),
		}

		folderIdx := b.uint16()
		if len(c.Folders) <= int(folderIdx) {
			return errors.New("folder index out of range")
		}

		_ = b.uint16() // date
		_ = b.uint16() // time

		file.attributes = b.uint16()

		file.Name = b.nullTerminatedString() // need to handle UTF-8...

		c.Folders[folderIdx].Files = append(c.Folders[folderIdx].Files, file)
		b.skip(int(dataReserveSize))
	}

	return b.err
}

// Ref is a reference to another cabinet.
type Ref struct {
	Disk string
	Name string
}

// Folder is metadata about a folder in a cabinet.
type Folder struct {
	Files []*File

	firstDataOffset uint32
	numDataBlocks   uint16
	compressionBits uint16
	compressionType uint8
}

// File is metadata about a file in a cabinet.
type File struct {
	Name     string
	DateTime time.Time

	uncompressedSize   uint32
	uncompressedOffset uint32
	attributes         uint16
}

type readBuf struct {
	buf  *bufio.Reader
	temp [4]byte
	err  error
}

func (b *readBuf) nullTerminatedString() (s string) {
	s, b.err = b.buf.ReadString(0x0)
	return s[:len(s)-1]
}

func (b *readBuf) skip(n int) {
	_, b.err = b.buf.Discard(n)
}

func (b *readBuf) uint8() uint8 {
	r, err := b.buf.ReadByte()
	b.err = err
	return r
}

func (b *readBuf) uint16() uint16 {
	_, b.err = b.buf.Read(b.temp[:2])
	return binary.LittleEndian.Uint16(b.temp[:2])
}

func (b *readBuf) uint32() uint32 {
	_, b.err = b.buf.Read(b.temp[:])
	return binary.LittleEndian.Uint32(b.temp[:])
}
