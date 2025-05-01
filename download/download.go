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
var ErrIdleTimeout = errors.New("download timed out: connection idle for too long")

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

	// Create HTTP client with timeouts to detect network failures faster
	client := &http.Client{
		Timeout: 5 * time.Second, // Overall timeout for the request
		Transport: &http.Transport{
			ResponseHeaderTimeout: 10 * time.Second, // Timeout for server response headers
			IdleConnTimeout:       5 * time.Second,  // Idle connection timeout
			TLSHandshakeTimeout:   5 * time.Second,  // TLS handshake timeout
			ExpectContinueTimeout: 1 * time.Second,  // Expect-continue timeout
			DisableKeepAlives:     false,            // Keep connections alive for efficiency
			MaxIdleConnsPerHost:   10,               // Maximum idle connections per host
		},
	}

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

	// Create a progress reader with idle timeout
	progressReader := &ProgressReader{
		Reader:           resp.Body,
		Callback:         progressCb,
		Total:            totalSize,
		CancelCh:         cancelCh,
		idleTimeout:      5 * time.Second, // Add idle timeout to detect stalled connections
		lastProgressTime: time.Now(),      // Initialize the last progress time
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
	Callback         ProgressCallback
	Current          int64
	Total            int64
	lastCallbackAt   int64           // Last reported byte count
	minReportBytes   int64           // Minimum bytes changed before reporting again
	lastReportTime   time.Time       // Last time progress was reported
	minReportRate    time.Duration   // Minimum time between reports
	CancelCh         <-chan struct{} // Added cancel channel
	idleTimeout      time.Duration   // Added idle timeout
	lastProgressTime time.Time       // Added last progress time
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

	// If we actually read data, update the last progress time
	if n > 0 {
		pr.lastProgressTime = time.Now()
	}

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

	// Check for idle timeout - only if we have an active timeout set
	if pr.idleTimeout > 0 && !pr.lastProgressTime.IsZero() && time.Since(pr.lastProgressTime) > pr.idleTimeout {
		return n, ErrIdleTimeout
	}

	return
}

// extractTarXz extracts a .tar.xz archive with progress updates.
func extractTarXz(archivePath, destDir string, progressCb ExtractionProgressCallback, cancelCh <-chan struct{}) error {
	// Get file info to calculate rough progress based on archive size
	fileInfo, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("failed to stat archive file: %w", err)
	}
	archiveSize := fileInfo.Size()

	file, err := os.Open(archivePath)
	if err != nil {
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

extractLoop:
	for {
		// Check for cancellation before processing next entry
		select {
		case <-cancelCh:
			setFirstError(ErrCancelled)
			break extractLoop
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
				setFirstError(fmt.Errorf("error reading tar entry: %w", err))
			}
			break extractLoop
		}
		entryCount++

		// Use header.Name as is without modifying the path
		targetPath := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				setFirstError(fmt.Errorf("failed to create dir %s: %w", targetPath, err))
				break extractLoop
			}
		case tar.TypeReg:
			if header.Size > 0 {
				if header.Size <= int64(bufferSize) {
					fileContents := make([]byte, header.Size)
					if _, err := io.ReadFull(tarReader, fileContents); err != nil {
						if errors.Is(err, ErrCancelled) {
							setFirstError(ErrCancelled)
						} else {
							setFirstError(fmt.Errorf("failed to read file contents for %s: %w", targetPath, err))
						}
						break extractLoop
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
							errChan <- fmt.Errorf("failed to create parent dir for file %s: %w", targetPath, err)
							return
						}

						if err := os.WriteFile(targetPath, contents, os.FileMode(fileMode)); err != nil {
							errChan <- fmt.Errorf("failed to write file %s: %w", targetPath, err)
							return
						}
					}(targetPath, header.Mode, fileContents)
				} else {
					if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
						setFirstError(fmt.Errorf("failed to create parent dir for file %s: %w", targetPath, err))
						break extractLoop
					}

					outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
					if err != nil {
						setFirstError(fmt.Errorf("failed to create file %s: %w", targetPath, err))
						break extractLoop
					}

					// Wrap tarReader with cancellation check
					cancelReader := &CancelableReader{Reader: tarReader, CancelCh: cancelCh}

					bufferedWriter := bufio.NewWriterSize(outFile, bufferSize)
					if _, err := io.CopyBuffer(bufferedWriter, cancelReader, copyBuffer); err != nil {
						outFile.Close()
						if errors.Is(err, ErrCancelled) {
							setFirstError(ErrCancelled)
						} else {
							setFirstError(fmt.Errorf("failed to write file %s: %w", targetPath, err))
						}
						break extractLoop
					}

					if err := bufferedWriter.Flush(); err != nil {
						outFile.Close()
						setFirstError(fmt.Errorf("failed to flush buffers for %s: %w", targetPath, err))
						break extractLoop
					}

					if err := outFile.Close(); err != nil {
						setFirstError(fmt.Errorf("failed to close file %s: %w", targetPath, err))
						break extractLoop
					}
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
					setFirstError(fmt.Errorf("failed to create parent dir for empty file %s: %w", targetPath, err))
					break extractLoop
				}

				if err := os.WriteFile(targetPath, []byte{}, os.FileMode(header.Mode)); err != nil {
					setFirstError(fmt.Errorf("failed to create empty file %s: %w", targetPath, err))
					break extractLoop
				}
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
				setFirstError(fmt.Errorf("failed to create parent dir for symlink %s: %w", targetPath, err))
				break extractLoop
			}
			if _, err := os.Lstat(targetPath); err == nil {
				if err := os.Remove(targetPath); err != nil {
					setFirstError(fmt.Errorf("failed to remove existing file/link at %s: %w", targetPath, err))
					break extractLoop
				}
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				setFirstError(fmt.Errorf("failed to create symlink %s -> %s: %w", targetPath, header.Linkname, err))
				break extractLoop
			}
		}
	}

	// Remove the cleanup label and just have the cleanup code
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
			if errRem := os.RemoveAll(existingBuildDir); errRem != nil {
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
