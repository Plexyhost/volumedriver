package compression

import (
	"archive/tar"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

func Compress(src string, dst io.Writer) error {
	// Writer chain
	// tar -> gzip -> dst
	// zr := gzip.NewWriter(dst)
	zr, _ := zstd.NewWriter(dst)
	tw := tar.NewWriter(zr)

	err := filepath.WalkDir(src, func(file string, e fs.DirEntry, _ error) error {
		// Construct header
		fi, err := e.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// Make the header name relative to the src directory
		header.Name, err = filepath.Rel(src, file)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(header.Name)

		// Ignore world/session.lock
		if header.Name == "world/session.lock" {
			return nil
		}

		// Write header through writer chain
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			defer data.Close()
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Close the writer chain
	if err := tw.Close(); err != nil {
		return err
	}

	return zr.Close()
}
