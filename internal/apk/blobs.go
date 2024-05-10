package apk

import (
	"bufio"
	"bytes"
	ogzip "compress/gzip"
	"fmt"
	"io"

	"github.com/xenon007/registry-explorer/internal/gzip"
	"github.com/xenon007/registry-explorer/internal/zstd"
)

// Pretends to implement Seek because ServeContent only cares about checking
// for the size by calling Seek(0, io.SeekEnd)
type sizeSeeker struct {
	rc   io.Reader
	size int64
}

func (s *sizeSeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (s *sizeSeeker) Read(p []byte) (int, error) {
	return s.rc.Read(p)
}

func (s *sizeSeeker) Size() int64 {
	return s.size
}

type sizeBlob struct {
	io.ReadCloser
	size int64
}

func (s *sizeBlob) Size() int64 {
	return s.size
}

const (
	magicGNU, versionGNU     = "ustar ", " \x00"
	magicUSTAR, versionUSTAR = "ustar\x00", "00"
)

func tarPeek(r io.Reader) (bool, gzip.PeekReader, error) {
	// Make sure it's more than 512
	var pr gzip.PeekReader
	if p, ok := r.(gzip.PeekReader); ok {
		pr = p
	} else {
		// For tar peek.
		pr = bufio.NewReaderSize(r, 1<<16)
	}

	block, err := pr.Peek(512)
	if err != nil {
		// https://github.com/google/go-containerregistry/issues/367
		if err == io.EOF {
			return false, pr, nil
		}
		return false, pr, err
	}

	magic := string(block[257:][:6])
	isTar := magic == magicGNU || magic == magicUSTAR
	return isTar, pr, nil
}

func gztarPeek(r io.Reader) (bool, gzip.PeekReader, error) {
	pr := bufio.NewReaderSize(r, 1<<16)

	// Should be enough to read first block?
	zb, err := pr.Peek(1024)
	if err != nil {
		if err != io.EOF {
			return false, pr, err
		}
	}

	br := bytes.NewReader(zb)
	ok, zpr, err := gzip.Peek(br)
	if !ok {
		return ok, pr, err
	}

	zr, err := ogzip.NewReader(zpr)
	if err != nil {
		return false, pr, err
	}
	ok, _, err = tarPeek(zr)
	return ok, pr, err
}

func zstdTarPeek(r io.Reader) (bool, gzip.PeekReader, error) {
	pr := bufio.NewReaderSize(r, 1<<16)

	// Should be enough to read first block?
	zb, err := pr.Peek(1024)
	if err != nil {
		if err != io.EOF {
			return false, pr, err
		}
	}

	br := bytes.NewReader(zb)
	ok, zpr, err := zstdPeek(br)
	if !ok {
		return ok, pr, err
	}

	zr, err := ogzip.NewReader(zpr)
	if err != nil {
		return false, pr, err
	}
	ok, _, err = tarPeek(zr)
	return ok, pr, err
}

func zstdPeek(r io.Reader) (bool, gzip.PeekReader, error) {
	// Make sure it's more than 512
	var pr gzip.PeekReader
	if p, ok := r.(gzip.PeekReader); ok {
		pr = p
	} else {
		// For tar peek.
		pr = bufio.NewReaderSize(r, 1<<16)
	}

	return checkHeader(pr, zstd.MagicHeader)
}

// CheckHeader checks whether the first bytes from a PeekReader match an expected header
func checkHeader(pr gzip.PeekReader, expectedHeader []byte) (bool, gzip.PeekReader, error) {
	header, err := pr.Peek(len(expectedHeader))
	if err != nil {
		// https://github.com/google/go-containerregistry/issues/367
		if err == io.EOF {
			return false, pr, nil
		}
		return false, pr, err
	}
	return bytes.Equal(header, expectedHeader), pr, nil
}
