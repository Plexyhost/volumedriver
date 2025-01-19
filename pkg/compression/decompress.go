package compression

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

func Decompress(src io.Reader, dst string) error {

	// Reader chain
	// src -> gzip -> tar
	// gr, err := gzip.NewReader(src)
	gr, err := zstd.NewReader(src)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	// Clear directory
	if err := os.RemoveAll(dst); err != nil {
		return err
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dst, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			// Create file
			file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return err
			}
			file.Close()
		default:
			return fmt.Errorf("unsupported type: %v", header.Typeflag)
		}
	}
	return nil
}
