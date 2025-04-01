package download

import (
	"TUI-Blender-Launcher/model"
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv" // Added for Content-Length parsing
	"strings"

	// Added for potential speed calculation later
	"github.com/ulikunitz/xz"
)

// versionMetaFilename is the name of the metadata file saved in the extracted directory.
const versionMetaFilename = "version.json"

// ProgressCallback is a function type for reporting download progress.
// It receives bytes downloaded and total file size.
type ProgressCallback func(downloadedBytes, totalBytes int64)

// downloadFile downloads a file, reporting progress via the callback.
func downloadFile(url, downloadPath string, progressCb ProgressCallback) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	// Add a user-agent? Some servers might require it.
	// req.Header.Set("User-Agent", "tui-blender-launcher")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	totalSizeStr := resp.Header.Get("Content-Length")
	totalSize, err := strconv.ParseInt(totalSizeStr, 10, 64)
	if err != nil || totalSize <= 0 {
		// If Content-Length is missing or invalid, we can't show percentage
		// Maybe fall back to a spinner or just download without progress?
		// For now, let's error out if we can't get size, as progress relies on it.
		return fmt.Errorf("could not determine file size from Content-Length header: %s", totalSizeStr)
	}

	// Ensure download directory exists
	if err := os.MkdirAll(filepath.Dir(downloadPath), 0750); err != nil {
		return fmt.Errorf("failed to create download dir: %w", err)
	}

	out, err := os.Create(downloadPath)
	if err != nil {
		return fmt.Errorf("failed to create download file: %w", err)
	}
	defer out.Close()

	// Create a progress reader
	progressReader := &ProgressReader{
		Reader:   resp.Body,
		Callback: progressCb,
		Total:    totalSize,
	}

	// Trigger initial callback
	if progressCb != nil {
		progressCb(0, totalSize)
	}

	_, err = io.Copy(out, progressReader)
	if err != nil {
		return fmt.Errorf("failed during download/save: %w", err)
	}

	// Ensure final callback is called
	if progressCb != nil {
		progressCb(progressReader.Current, totalSize)
	}

	return nil
}

// ProgressReader wraps an io.Reader to report progress via a callback.
type ProgressReader struct {
	io.Reader
	Callback ProgressCallback
	Current  int64
	Total    int64
	// Add fields for throttling callbacks if needed
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.Current += int64(n)
	if pr.Callback != nil {
		// TODO: Throttle callback? e.g., only call every X ms or Y bytes
		pr.Callback(pr.Current, pr.Total)
	}
	return
}

// extractTarXz extracts a .tar.xz archive.
func extractTarXz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	xzReader, err := xz.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create xz reader: %w", err)
	}

	tarReader := tar.NewReader(xzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed read tar header: %w", err)
		}

		targetPath := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create dir %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
				return fmt.Errorf("failed to create parent dir for file %s: %w", targetPath, err)
			}
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close() // Close file before returning error
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			outFile.Close() // Close file successfully
		default:
			// Handle other types like symlinks if necessary
			// fmt.Printf("Skipping unsupported type %c for %s\n", header.Typeflag, header.Name)
		}
	}

	return nil
}

// TODO: Add extractZip function if needed for other OS

// saveVersionMetadata saves the build info as version.json inside the extracted directory.
func saveVersionMetadata(build model.BlenderBuild, extractedDir string) error {
	metaPath := filepath.Join(extractedDir, versionMetaFilename)
	jsonData, err := json.MarshalIndent(build, "", "  ") // Pretty print JSON
	if err != nil {
		return fmt.Errorf("failed to marshal build metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", versionMetaFilename, err)
	}
	return nil
}

// DownloadAndExtractBuild modified to accept progress callback channel from TUI
// Note: Direct channel passing isn't ideal here. It's better if the command
// itself manages the goroutine and progress reporting back to the TUI model.
// Let's keep the signature simple for now and handle progress in the TUI command.
func DownloadAndExtractBuild(build model.BlenderBuild, downloadBaseDir string, progressCb ProgressCallback) (string, error) {
	// 1. Download
	downloadFileName := filepath.Base(build.DownloadURL)
	downloadPath := filepath.Join(downloadBaseDir, ".downloading", downloadFileName)

	if err := downloadFile(build.DownloadURL, downloadPath, progressCb); err != nil {
		// Attempt cleanup even if download fails
		os.Remove(downloadPath)
		os.Remove(filepath.Dir(downloadPath))
		return "", fmt.Errorf("download failed: %w", err)
	}

	// 2. Determine Extraction Directory
	extractedDirName := strings.TrimSuffix(strings.TrimSuffix(downloadFileName, ".tar.xz"), ".zip")
	extractedPath := filepath.Join(downloadBaseDir, extractedDirName)

	if _, err := os.Stat(extractedPath); err == nil {
		if err := os.RemoveAll(extractedPath); err != nil {
			return "", fmt.Errorf("failed to remove existing target directory %s: %w", extractedPath, err)
		}
	}

	// 3. Extract
	var extractErr error
	switch build.FileExtension {
	case "xz", "tar.xz":
		extractErr = extractTarXz(downloadPath, downloadBaseDir)
	case "zip":
		extractErr = fmt.Errorf("zip extraction not yet implemented")
	default:
		extractErr = fmt.Errorf("unsupported archive type: %s", build.FileExtension)
	}

	// 4. Delete downloaded archive
	if err := os.Remove(downloadPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to delete archive %s: %v\n", downloadPath, err)
	}
	os.Remove(filepath.Dir(downloadPath))

	if extractErr != nil {
		os.RemoveAll(extractedPath) // Clean up partial extraction
		return "", fmt.Errorf("extraction failed: %w", extractErr)
	}

	// 5. Save Metadata
	if err := saveVersionMetadata(build, extractedPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save version metadata: %v\n", err)
	}

	return extractedPath, nil
}
