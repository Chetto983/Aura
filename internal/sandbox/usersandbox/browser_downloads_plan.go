package usersandbox

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"strings"
)

func downloadKey(file browserDownload) string {
	return fmt.Sprintf("%s/%d/%d", file.Name, file.Size, file.MTime)
}

// planBrowserDownloads picks the next batch to copy and the notices for the model. delivered maps
// each listed file already dealt with to its shell path, or to "" for an oversized file the
// model has been told about once; remember reports a new such mark, which the checkpoint must
// keep even when nothing is copied.
func planBrowserDownloads(inventory browserDownloadInventory) (files []browserDownload, delivered map[string]string, notices string, remember bool) {
	delivered = make(map[string]string)
	var footer strings.Builder
	noticed, totalBytes := 0, int64(0)
	for _, file := range inventory.Files {
		if !validDownloadName(file.Name) || file.Size < 0 {
			continue
		}
		key := downloadKey(file)
		if destination, done := inventory.Delivered[key]; done {
			delivered[key] = destination
			continue
		}
		if strings.HasSuffix(file.Name, ".crdownload") || file.Size > maxBrowserDownloadBytes {
			if noticed < 8 {
				if strings.HasSuffix(file.Name, ".crdownload") {
					fmt.Fprintf(&footer, "\nDownload in progress: %s (%d bytes).", file.Name, file.Size)
				} else {
					fmt.Fprintf(&footer, "\nDownload not copied: %s exceeds the 50 MiB limit.", file.Name)
					delivered[key] = ""
					remember = true
				}
				noticed++
			}
			continue
		}
		if len(files) >= 16 || totalBytes+file.Size > 100<<20 {
			continue
		}
		files = append(files, file)
		totalBytes += file.Size
	}
	return files, delivered, footer.String(), remember
}

func copyBrowserDownloadArchive(src io.Reader, dst io.Writer, file browserDownload) error {
	if !validDownloadName(file.Name) || file.Size < 0 || file.Size > maxBrowserDownloadBytes {
		return errors.New("browser download name or size refused")
	}
	r := tar.NewReader(src)
	header, err := r.Next()
	if err != nil {
		return err
	}
	if header.Name != file.Name || header.Typeflag != tar.TypeReg || header.Size != file.Size {
		return errors.New("browser download is not the expected regular file")
	}
	if _, err := io.CopyN(dst, r, file.Size); err != nil {
		return err
	}
	if _, err := r.Next(); !errors.Is(err, io.EOF) {
		return errors.New("browser download contains extra archive entries")
	}
	return nil
}
