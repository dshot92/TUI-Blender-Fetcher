package download

import (
	"TUI-Blender-Launcher/model"
	"archive/tar"
	"archive/zip"
	"bufio"
	"context" // Import context package
	"encoding/json"
	"errors" // Import errors package
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv" // Added for Content-Length parsing
	"strings"
	"sync"
	"time"

	// Added for potential speed calculation later
	"github.com/ulikunitz/xz"
)

// Error constants
var ErrCancelled = errors.New("operation cancelled")

// versionMetaFilename is the name of the metadata file saved in the extracted directory.
const versionMetaFilename = "version.json"

// ProgressCallback is a function type for reporting download progress.
// It receives bytes downloaded and total file size.
type ProgressCallback func(downloadedBytes, totalBytes int64)

// ExtractionProgressCallback represents a callback used to report extraction progress.
// Since we can't know the total size up front, we use a percentage (0.0-1.0) estimate.
type ExtractionProgressCallback func(estimatedProgress float64)

// downloadFile downloads a file, reporting progress via the callback.
func downloadFile(url, downloadPath string, progressCb ProgressCallback, cancelCh <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-cancelCh:
			cancel() // Cancel the context if the channel is closed
		case <-ctx.Done():
			// Context finished normally
		}
	}()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil) // Use request with context
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return ErrCancelled
		}
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	totalSizeStr := resp.Header.Get("Content-Length")
	totalSize, err := strconv.ParseInt(totalSizeStr, 10, 64)
	if err != nil || totalSize <= 0 {
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
		CancelCh: cancelCh,
	}

	// Trigger initial callback
	if progressCb != nil {
		progressCb(0, totalSize)
	}

	// Use a buffer for copying
	copyBuffer := make([]byte, 32*1024)

	_, err = io.CopyBuffer(out, progressReader, copyBuffer)
	if err != nil {
		if errors.Is(err, ErrCancelled) {
			return ErrCancelled
		}
		return fmt.Errorf("failed during download/save: %w", err)
	}

	// Ensure final callback is called if not cancelled
	if progressCb != nil {
		progressCb(progressReader.Current, totalSize)
	}

	return nil
}

// ProgressReader wraps an io.Reader to report progress via a callback.
type ProgressReader struct {
	io.Reader
	Callback       ProgressCallback
	Current        int64
	Total          int64
	lastCallbackAt int64           // Last reported byte count
	minReportBytes int64           // Minimum bytes changed before reporting again
	lastReportTime time.Time       // Last time progress was reported
	minReportRate  time.Duration   // Minimum time between reports
	CancelCh       <-chan struct{} // Added cancel channel
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	// Check for cancellation before reading
	select {
	case <-pr.CancelCh:
		return 0, ErrCancelled
	default:
		// Continue
	}

	n, err = pr.Reader.Read(p)
	pr.Current += int64(n)

	// Initialize throttling values if not set
	if pr.minReportBytes == 0 {
		pr.minReportBytes = 100 * 1024
	}
	if pr.minReportRate == 0 {
		pr.minReportRate = 100 * time.Millisecond
	}
	if pr.lastReportTime.IsZero() {
		pr.lastReportTime = time.Now()
	}

	if pr.Callback != nil {
		bytesSinceLastCallback := pr.Current - pr.lastCallbackAt
		timeSinceLastCallback := time.Since(pr.lastReportTime)

		if bytesSinceLastCallback >= pr.minReportBytes ||
			timeSinceLastCallback >= pr.minReportRate ||
			pr.Current == pr.Total || err == io.EOF {
			pr.Callback(pr.Current, pr.Total)
			pr.lastCallbackAt = pr.Current
			pr.lastReportTime = time.Now()
		}
	}

	// Check for cancellation again after reading, in case it happened during the read
	select {
	case <-pr.CancelCh:
		return n, ErrCancelled
	default:
		// Continue
	}
	return
}

// extractTarXz extracts a .tar.xz archive with progress updates.
func extractTarXz(archivePath, destDir string, progressCb ExtractionProgressCallback, cancelCh <-chan struct{}) error {
	// Get file info to calculate rough progress based on archive size
	fileInfo, err := os.Stat(archivePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stat archive file: %v\n", err)
		return fmt.Errorf("failed to stat archive file: %w", err)
	}
	archiveSize := fileInfo.Size()

	file, err := os.Open(archivePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open archive: %v\n", err)
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	// Increase buffer size for better performance
	const bufferSize = 4 * 1024 * 1024 // 4MB buffer for better throughput
	bufferedFile := bufio.NewReaderSize(file, bufferSize)

	// Create a reader that will track read progress with throttling
	progressReader := &ProgressReader{
		Reader:         bufferedFile,
		Total:          archiveSize,
		Current:        0,
		lastCallbackAt: 0,
		minReportBytes: 256 * 1024,             // Report every 256KB during extraction
		minReportRate:  200 * time.Millisecond, // Report at most 5 times per second
		CancelCh:       cancelCh,               // Pass cancel channel
		Callback: func(read, total int64) {
			if progressCb != nil {
				// Convert to estimated extraction progress (0.0-1.0)
				estimatedProgress := float64(read) / float64(total)
				progressCb(estimatedProgress)
			}
		},
	}

	xzReader, err := xz.NewReader(progressReader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create xz reader: %v\n", err)
		return fmt.Errorf("failed to create xz reader: %w", err)
	}

	bufferedXzReader := bufio.NewReaderSize(xzReader, bufferSize)
	tarReader := tar.NewReader(bufferedXzReader)

	copyBuffer := make([]byte, bufferSize)

	if progressCb != nil {
		progressCb(0.0)
	}

	const maxWorkers = 4
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	errChan := make(chan error, maxWorkers)
	var firstErr error
	var errLock sync.Mutex

	// Function to set the first error encountered
	setFirstError := func(err error) {
		errLock.Lock()
		if firstErr == nil && err != nil {
			firstErr = err
		}
		errLock.Unlock()
	}

	var entryCount int
	for {
		// Check for cancellation before processing next entry
		select {
		case <-cancelCh:
			setFirstError(ErrCancelled)
			goto cleanup // Jump to cleanup section
		default:
		}

		// Check if an error occurred in workers
		errLock.Lock()
		errOccurred := firstErr
		errLock.Unlock()
		if errOccurred != nil {
			break
		}

		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			if errors.Is(err, ErrCancelled) {
				setFirstError(ErrCancelled)
			} else {
				fmt.Fprintf(os.Stderr, "Error reading tar entry: %v\n", err)
				setFirstError(fmt.Errorf("error reading tar entry: %w", err))
			}
			break
		}
		entryCount++

		// Use header.Name as is without modifying the path
		targetPath := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create dir %s: %v\n", targetPath, err)
				setFirstError(fmt.Errorf("failed to create dir %s: %w", targetPath, err))
				break
			}
		case tar.TypeReg:
			if header.Size > 0 {
				if header.Size <= int64(bufferSize) {
					fileContents := make([]byte, header.Size)
					if _, err := io.ReadFull(tarReader, fileContents); err != nil {
						if errors.Is(err, ErrCancelled) {
							setFirstError(ErrCancelled)
						} else {
							fmt.Fprintf(os.Stderr, "Failed to read file contents for %s: %v\n", targetPath, err)
							setFirstError(fmt.Errorf("failed to read file contents for %s: %w", targetPath, err))
						}
						break
					}

					wg.Add(1)
					go func(targetPath string, fileMode int64, contents []byte) {
						defer wg.Done()
						select {
						case sem <- struct{}{}: // Acquire semaphore
							defer func() { <-sem }() // Release semaphore
						case <-cancelCh:
							errChan <- ErrCancelled
							return
						}

						if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
							fmt.Fprintf(os.Stderr, "Failed to create parent dir for file %s: %v\n", targetPath, err)
							errChan <- fmt.Errorf("failed to create parent dir for file %s: %w", targetPath, err)
							return
						}

						if err := os.WriteFile(targetPath, contents, os.FileMode(fileMode)); err != nil {
							fmt.Fprintf(os.Stderr, "Failed to write file %s: %v\n", targetPath, err)
							errChan <- fmt.Errorf("failed to write file %s: %w", targetPath, err)
							return
						}
					}(targetPath, header.Mode, fileContents)
				} else {
					if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to create parent dir for file %s: %v\n", targetPath, err)
						setFirstError(fmt.Errorf("failed to create parent dir for file %s: %w", targetPath, err))
						break
					}

					outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
					if err != nil {
						fmt.Fprintf(os.Stderr, "Failed to create file %s: %v\n", targetPath, err)
						setFirstError(fmt.Errorf("failed to create file %s: %w", targetPath, err))
						break
					}

					// Wrap tarReader with cancellation check
					cancelReader := &CancelableReader{Reader: tarReader, CancelCh: cancelCh}

					bufferedWriter := bufio.NewWriterSize(outFile, bufferSize)
					if _, err := io.CopyBuffer(bufferedWriter, cancelReader, copyBuffer); err != nil {
						outFile.Close()
						if errors.Is(err, ErrCancelled) {
							setFirstError(ErrCancelled)
						} else {
							fmt.Fprintf(os.Stderr, "Failed to write file %s: %v\n", targetPath, err)
							setFirstError(fmt.Errorf("failed to write file %s: %w", targetPath, err))
						}
						break
					}

					if err := bufferedWriter.Flush(); err != nil {
						outFile.Close()
						fmt.Fprintf(os.Stderr, "Failed to flush buffers for %s: %v\n", targetPath, err)
						setFirstError(fmt.Errorf("failed to flush buffers for %s: %w", targetPath, err))
						break
					}

					if err := outFile.Close(); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to close file %s: %v\n", targetPath, err)
						setFirstError(fmt.Errorf("failed to close file %s: %w", targetPath, err))
						break
					}
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to create parent dir for empty file %s: %v\n", targetPath, err)
					setFirstError(fmt.Errorf("failed to create parent dir for empty file %s: %w", targetPath, err))
					break
				}

				if err := os.WriteFile(targetPath, []byte{}, os.FileMode(header.Mode)); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to create empty file %s: %v\n", targetPath, err)
					setFirstError(fmt.Errorf("failed to create empty file %s: %w", targetPath, err))
					break
				}
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create parent dir for symlink %s: %v\n", targetPath, err)
				setFirstError(fmt.Errorf("failed to create parent dir for symlink %s: %w", targetPath, err))
				break
			}
			if _, err := os.Lstat(targetPath); err == nil {
				if err := os.Remove(targetPath); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to remove existing file/link at %s: %v\n", targetPath, err)
					setFirstError(fmt.Errorf("failed to remove existing file/link at %s: %w", targetPath, err))
					break
				}
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create symlink %s -> %s: %v\n", targetPath, header.Linkname, err)
				setFirstError(fmt.Errorf("failed to create symlink %s -> %s: %w", targetPath, header.Linkname, err))
				break
			}
		}
	}

cleanup:
	wg.Wait()
	close(errChan)
	for err := range errChan {
		setFirstError(err)
	}

	if progressCb != nil {
		progressCb(1.0)
	}

	return firstErr
}

// CancelableReader wraps an io.Reader and checks a cancel channel.
type CancelableReader struct {
	io.Reader
	CancelCh <-chan struct{}
}

func (r *CancelableReader) Read(p []byte) (n int, err error) {
	select {
	case <-r.CancelCh:
		return 0, ErrCancelled
	default:
		return r.Reader.Read(p)
	}
}

// saveVersionMetadata saves the build info as version.json inside the extracted directory.
func saveVersionMetadata(build model.BlenderBuild, extractedDir string) error {
	metaPath := filepath.Join(extractedDir, versionMetaFilename)

	if build.BuildDate.Time().IsZero() {
		build.BuildDate = model.Timestamp(time.Now())
	}

	jsonData, err := json.MarshalIndent(build, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal build metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", versionMetaFilename, err)
	}
	return nil
}

// extractZip extracts a .zip archive with progress updates.
func extractZip(archivePath, destDir string, progressCb ExtractionProgressCallback, cancelCh <-chan struct{}) error {
	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip archive: %w", err)
	}
	defer zipReader.Close()

	// Get total uncompressed size for progress tracking
	var totalSize uint64
	for _, file := range zipReader.File {
		totalSize += file.UncompressedSize64
	}

	// Create a buffer for copying file contents
	const bufferSize = 4 * 1024 * 1024 // 4MB buffer
	copyBuffer := make([]byte, bufferSize)

	if progressCb != nil {
		progressCb(0.0)
	}

	var processedSize uint64
	var processedSizeLock sync.Mutex

	const maxWorkers = 4
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	errChan := make(chan error, maxWorkers)
	var firstErr error
	var errLock sync.Mutex

	// Function to set the first error encountered
	setFirstError := func(err error) {
		errLock.Lock()
		if firstErr == nil && err != nil {
			firstErr = err
		}
		errLock.Unlock()
	}

	for i, file := range zipReader.File {
		// Check for cancellation before processing next file
		select {
		case <-cancelCh:
			setFirstError(ErrCancelled)
			goto cleanup
		default:
		}

		// Check if an error occurred in workers
		errLock.Lock()
		errOccurred := firstErr
		errLock.Unlock()
		if errOccurred != nil {
			break
		}

		// Get proper file path ensuring no path traversal
		targetPath := filepath.Join(destDir, file.Name)

		// Make sure we follow zip entry slashes on Windows
		targetPath = filepath.FromSlash(targetPath)

		if file.FileInfo().IsDir() {
			// Create directory
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				setFirstError(fmt.Errorf("failed to create directory %s: %w", targetPath, err))
				break
			}
			continue
		}

		// Make sure parent directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
			setFirstError(fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err))
			break
		}

		// Small files can be read entirely into memory
		if file.UncompressedSize64 <= uint64(bufferSize) {
			wg.Add(1)
			go func(file *zip.File, targetPath string) {
				defer wg.Done()
				select {
				case sem <- struct{}{}: // Acquire semaphore
					defer func() { <-sem }() // Release semaphore
				case <-cancelCh:
					errChan <- ErrCancelled
					return
				}

				rc, err := file.Open()
				if err != nil {
					errChan <- fmt.Errorf("failed to open zip file entry %s: %w", file.Name, err)
					return
				}
				defer rc.Close()

				fileContents := make([]byte, file.UncompressedSize64)
				if _, err := io.ReadFull(rc, fileContents); err != nil {
					errChan <- fmt.Errorf("failed to read zip file entry %s: %w", file.Name, err)
					return
				}

				if err := os.WriteFile(targetPath, fileContents, file.Mode()); err != nil {
					errChan <- fmt.Errorf("failed to write file %s: %w", targetPath, err)
					return
				}

				// Update processed size for progress reporting
				processedSizeLock.Lock()
				processedSize += file.UncompressedSize64
				currentSize := processedSize
				processedSizeLock.Unlock()

				if progressCb != nil && totalSize > 0 {
					progressCb(float64(currentSize) / float64(totalSize))
				}
			}(file, targetPath)
		} else {
			// Larger files are processed in the main goroutine
			rc, err := file.Open()
			if err != nil {
				setFirstError(fmt.Errorf("failed to open zip file entry %s: %w", file.Name, err))
				break
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, file.Mode())
			if err != nil {
				rc.Close()
				setFirstError(fmt.Errorf("failed to create file %s: %w", targetPath, err))
				break
			}

			// Wrap reader with cancellation check
			cancelReader := &CancelableReader{Reader: rc, CancelCh: cancelCh}

			written, err := io.CopyBuffer(outFile, cancelReader, copyBuffer)
			outFile.Close()
			rc.Close()

			if err != nil {
				if errors.Is(err, ErrCancelled) {
					setFirstError(ErrCancelled)
				} else {
					setFirstError(fmt.Errorf("failed to extract file %s: %w", targetPath, err))
				}
				break
			}

			// Update processed size for progress reporting
			processedSizeLock.Lock()
			processedSize += uint64(written)
			currentSize := processedSize
			processedSizeLock.Unlock()

			if progressCb != nil && totalSize > 0 {
				progressCb(float64(currentSize) / float64(totalSize))
			}
		}

		// Report progress periodically
		if i%10 == 0 && progressCb != nil && totalSize > 0 {
			processedSizeLock.Lock()
			currentSize := processedSize
			processedSizeLock.Unlock()
			progressCb(float64(currentSize) / float64(totalSize))
		}
	}

cleanup:
	wg.Wait()
	close(errChan)
	for err := range errChan {
		setFirstError(err)
	}

	if progressCb != nil {
		progressCb(1.0)
	}

	return firstErr
}

// findRootDirInZip peeks into the ZIP archive to find the root directory name
func findRootDirInZip(archivePath string) (string, error) {
	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip archive: %w", err)
	}
	defer zipReader.Close()

	if len(zipReader.File) == 0 {
		return "", fmt.Errorf("empty archive")
	}

	// Get the first entry and extract the root directory
	firstEntry := zipReader.File[0].Name
	parts := strings.Split(firstEntry, "/")
	if len(parts) > 0 {
		return parts[0], nil
	}

	return "", fmt.Errorf("no root directory found in archive")
}

// findRootDirInTarXz peeks into the archive to find the root directory name
func findRootDirInTarXz(archivePath string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	xzReader, err := xz.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("failed to create xz reader: %w", err)
	}

	tarReader := tar.NewReader(xzReader)

	// Read the first header
	header, err := tarReader.Next()
	if err != nil {
		if err == io.EOF {
			return "", fmt.Errorf("empty archive")
		}
		return "", fmt.Errorf("error reading tar header: %w", err)
	}

	// Extract the root directory from the path
	rootPath := header.Name
	parts := strings.Split(rootPath, "/")
	if len(parts) > 0 {
		return parts[0], nil
	}

	return "", fmt.Errorf("no root directory found in archive")
}

// DownloadAndExtractBuild downloads and extracts a build, handling cancellation.
func DownloadAndExtractBuild(build model.BlenderBuild, downloadBaseDir string, progressCb ProgressCallback, cancelCh <-chan struct{}) (string, error) {
	// 1. Download
	downloadFileName := filepath.Base(build.DownloadURL)
	downloadTempDir := filepath.Join(downloadBaseDir, ".downloading")
	if err := os.MkdirAll(downloadTempDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create download temp dir: %w", err)
	}
	downloadPath := filepath.Join(downloadTempDir, downloadFileName)

	// Defer cleanup of the downloaded archive file
	defer func() {
		if err := os.Remove(downloadPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete archive %s: %v\n", downloadPath, err)
		}
	}()

	if err := downloadFile(build.DownloadURL, downloadPath, progressCb, cancelCh); err != nil {
		if errors.Is(err, ErrCancelled) {
			return "", ErrCancelled // Propagate cancellation error
		}
		return "", fmt.Errorf("download failed: %w", err)
	}

	// Check for cancellation after download, before extraction
	select {
	case <-cancelCh:
		return "", ErrCancelled
	default:
		// Continue
	}

	// 2. The archive contains a root directory, we'll extract directly to downloadBaseDir
	// Look for any existing directory with this build version
	var existingBuildDir string
	entries, err := os.ReadDir(downloadBaseDir)
	if err == nil {
		// Find any directories that might contain this version
		version := build.Version
		for _, entry := range entries {
			if entry.IsDir() && entry.Name() != ".downloading" && entry.Name() != ".oldbuilds" {
				// Check if this directory contains the version we're downloading
				if strings.Contains(entry.Name(), version) {
					existingBuildDir = filepath.Join(downloadBaseDir, entry.Name())
					break
				}
			}
		}
	}

	// If we found an existing build directory, back it up
	if existingBuildDir != "" {
		oldBuildsDir := filepath.Join(downloadBaseDir, ".oldbuilds")
		if err := os.MkdirAll(oldBuildsDir, 0750); err != nil {
			return "", fmt.Errorf("failed to create .oldbuilds directory: %w", err)
		}
		timestamp := time.Now().Format("20060102_150405")
		oldBuildName := fmt.Sprintf("%s_%s", filepath.Base(existingBuildDir), timestamp)
		oldBuildPath := filepath.Join(oldBuildsDir, oldBuildName)
		if err := os.Rename(existingBuildDir, oldBuildPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: couldn't move old build to backup dir: %v\n", err)
			if errRem := os.RemoveAll(existingBuildDir); errRem != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove old build dir after failed move: %v\n", errRem)
				return "", fmt.Errorf("failed to replace old build dir: %w", err)
			}
		}
	}

	// 3. Extract based on archive type
	extractionCb := func(progress float64) {
		if progressCb != nil {
			// Use a large virtual size to indicate extraction phase to the UI
			const extractionVirtualSize int64 = 100 * 1024 * 1024
			currentBytes := int64(progress * float64(extractionVirtualSize))
			progressCb(currentBytes, extractionVirtualSize)
		}
	}

	var extractedRootDir string
	var extractErr error

	// Handle different archive formats
	if strings.HasSuffix(downloadFileName, ".tar.xz") {
		// Peek into the archive to find the root directory
		rootDir, err := findRootDirInTarXz(downloadPath)
		if err != nil {
			return "", fmt.Errorf("failed to find root directory in archive: %w", err)
		}
		extractedRootDir = filepath.Join(downloadBaseDir, rootDir)

		// Extract the archive
		extractErr = extractTarXz(downloadPath, downloadBaseDir, extractionCb, cancelCh)
	} else if strings.HasSuffix(downloadFileName, ".zip") {
		// Peek into the archive to find the root directory
		rootDir, err := findRootDirInZip(downloadPath)
		if err != nil {
			return "", fmt.Errorf("failed to find root directory in zip archive: %w", err)
		}
		extractedRootDir = filepath.Join(downloadBaseDir, rootDir)

		// Extract the zip archive
		extractErr = extractZip(downloadPath, downloadBaseDir, extractionCb, cancelCh)
	} else {
		return "", fmt.Errorf("unsupported archive format: %s", downloadFileName)
	}

	// Handle extraction error
	if extractErr != nil {
		// Attempt to clean up partially extracted directory
		if extractedRootDir != "" {
			if remErr := os.RemoveAll(extractedRootDir); remErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to cleanup partial extraction dir %s: %v\n", extractedRootDir, remErr)
			}
		}
		if errors.Is(extractErr, ErrCancelled) {
			return "", ErrCancelled // Propagate cancellation
		}
		return "", fmt.Errorf("extraction failed: %w", extractErr)
	}

	// 4. Save Metadata
	if err := saveVersionMetadata(build, extractedRootDir); err != nil {
		return extractedRootDir, fmt.Errorf("metadata save failed: %w", err)
	}

	return extractedRootDir, nil
}
