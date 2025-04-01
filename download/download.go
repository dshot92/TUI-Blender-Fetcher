package download

import (
	"TUI-Blender-Launcher/model"
	"archive/tar"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv" // Added for Content-Length parsing
	"strings"
	"time"
	"sync"

	// Added for potential speed calculation later
	"github.com/ulikunitz/xz"
)

// versionMetaFilename is the name of the metadata file saved in the extracted directory.
const versionMetaFilename = "version.json"

// ProgressCallback is a function type for reporting download progress.
// It receives bytes downloaded and total file size.
type ProgressCallback func(downloadedBytes, totalBytes int64)

// ExtractionProgressCallback represents a callback used to report extraction progress.
// Since we can't know the total size up front, we use a percentage (0.0-1.0) estimate.
type ExtractionProgressCallback func(estimatedProgress float64)

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
	Callback       ProgressCallback
	Current        int64
	Total          int64
	lastCallbackAt int64     // Last reported byte count
	minReportBytes int64     // Minimum bytes changed before reporting again
	lastReportTime time.Time // Last time progress was reported
	minReportRate  time.Duration // Minimum time between reports
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.Current += int64(n)
	
	// Initialize throttling values if not set
	if pr.minReportBytes == 0 {
		// Call progress at most every 100KB (to reduce UI updates)
		pr.minReportBytes = 100 * 1024
	}
	if pr.minReportRate == 0 {
		// Call progress at most every 100ms
		pr.minReportRate = 100 * time.Millisecond
	}
	if pr.lastReportTime.IsZero() {
		pr.lastReportTime = time.Now()
	}
	
	if pr.Callback != nil {
		// Only call the callback if:
		// 1. Enough bytes have been read since last update
		// 2. Enough time has passed since last update
		// 3. It's the first or last update (Current == 0 or err == io.EOF)
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
	return
}

// extractTarXz extracts a .tar.xz archive with progress updates.
func extractTarXz(archivePath, destDir string, progressCb ExtractionProgressCallback) error {
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
		Reader:   bufferedFile,
		Total:    archiveSize,
		Current:  0,
		lastCallbackAt: 0,
		minReportBytes: 256 * 1024, // Report every 256KB during extraction
		minReportRate: 200 * time.Millisecond, // Report at most 5 times per second
		Callback: func(read, total int64) {
			if progressCb != nil {
				// Convert to estimated extraction progress (0.0-1.0)
				// This is just an estimate based on compressed data read
				estimatedProgress := float64(read) / float64(total)
				progressCb(estimatedProgress)
			}
		},
	}

	// Create xz reader with improved concurrency
	// The xz.Reader doesn't natively support concurrency but we can improve other aspects
	xzReader, err := xz.NewReader(progressReader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create xz reader: %v\n", err)
		return fmt.Errorf("failed to create xz reader: %w", err)
	}

	// Wrap in a buffered reader for more efficient tar reading
	bufferedXzReader := bufio.NewReaderSize(xzReader, bufferSize)
	tarReader := tar.NewReader(bufferedXzReader)

	// Prepare a buffer for file copying
	copyBuffer := make([]byte, bufferSize)

	// Call with initial progress
	if progressCb != nil {
		progressCb(0.0)
	}
	
	// Create extraction worker pool to handle file I/O in parallel
	// This helps distribute CPU and I/O wait, improving throughput
	const maxWorkers = 4
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	
	// Create an error channel to track errors across goroutines
	errChan := make(chan error, maxWorkers)
	var extractionErr error

	// Process the tar header entries
	var entryCount int
	for extractionErr == nil {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading tar entry: %v\n", err)
			extractionErr = fmt.Errorf("error reading tar entry: %w", err)
			break
		}
		entryCount++

		targetPath := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create dir %s: %v\n", targetPath, err)
				extractionErr = fmt.Errorf("failed to create dir %s: %w", targetPath, err)
				break
			}
		case tar.TypeReg:
			// For regular files, use a worker pool to handle I/O in parallel
			if header.Size > 0 {
				// Copy file content to a buffer to free up the tar reader
				// Only do this for files of reasonable size
				if header.Size <= int64(bufferSize) {  // Reduced from bufferSize*2 to avoid memory pressure
					fileContents := make([]byte, header.Size)
					if _, err := io.ReadFull(tarReader, fileContents); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to read file contents for %s: %v\n", targetPath, err)
						extractionErr = fmt.Errorf("failed to read file contents for %s: %w", targetPath, err)
						break
					}
					
					// Process file in parallel worker
					wg.Add(1)
					go func(targetPath string, fileMode int64, contents []byte) {
						defer wg.Done()
						sem <- struct{}{} // Acquire semaphore
						defer func() { <-sem }() // Release semaphore
						
						// Ensure parent directory exists
						if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
							fmt.Fprintf(os.Stderr, "Failed to create parent dir for file %s: %v\n", targetPath, err)
							errChan <- fmt.Errorf("failed to create parent dir for file %s: %w", targetPath, err)
							return
						}
						
						// Write file
						if err := os.WriteFile(targetPath, contents, os.FileMode(fileMode)); err != nil {
							fmt.Fprintf(os.Stderr, "Failed to write file %s: %v\n", targetPath, err)
							errChan <- fmt.Errorf("failed to write file %s: %w", targetPath, err)
							return
						}
					}(targetPath, header.Mode, fileContents)
				} else {
					// For larger files, process them sequentially to avoid memory pressure
					// Ensure parent directory exists
					if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to create parent dir for file %s: %v\n", targetPath, err)
						extractionErr = fmt.Errorf("failed to create parent dir for file %s: %w", targetPath, err)
						break
					}
					
					// Create the file with appropriate permissions
					outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
					if err != nil {
						fmt.Fprintf(os.Stderr, "Failed to create file %s: %v\n", targetPath, err)
						extractionErr = fmt.Errorf("failed to create file %s: %w", targetPath, err)
						break
					}
					
					// Use buffered IO for better performance
					bufferedWriter := bufio.NewWriterSize(outFile, bufferSize)
					
					// Use copyBuffer for more efficient copying
					if _, err := io.CopyBuffer(bufferedWriter, tarReader, copyBuffer); err != nil {
						outFile.Close() // Close file before returning error
						fmt.Fprintf(os.Stderr, "Failed to write file %s: %v\n", targetPath, err)
						extractionErr = fmt.Errorf("failed to write file %s: %w", targetPath, err)
						break
					}
					
					// Flush the buffered writer to ensure all data is written
					if err := bufferedWriter.Flush(); err != nil {
						outFile.Close()
						fmt.Fprintf(os.Stderr, "Failed to flush buffers for %s: %v\n", targetPath, err)
						extractionErr = fmt.Errorf("failed to flush buffers for %s: %w", targetPath, err)
						break
					}
					
					// Close file successfully
					if err := outFile.Close(); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to close file %s: %v\n", targetPath, err)
						extractionErr = fmt.Errorf("failed to close file %s: %w", targetPath, err)
						break
					}
				}
			} else {
				// Empty file, just create it
				if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to create parent dir for empty file %s: %v\n", targetPath, err)
					extractionErr = fmt.Errorf("failed to create parent dir for empty file %s: %w", targetPath, err)
					break
				}
				
				if err := os.WriteFile(targetPath, []byte{}, os.FileMode(header.Mode)); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to create empty file %s: %v\n", targetPath, err)
					extractionErr = fmt.Errorf("failed to create empty file %s: %w", targetPath, err)
					break
				}
			}
		case tar.TypeSymlink:
			// Handle symlinks
			if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create parent dir for symlink %s: %v\n", targetPath, err)
				extractionErr = fmt.Errorf("failed to create parent dir for symlink %s: %w", targetPath, err)
				break
			}
			// Remove existing file or link if it exists
			if _, err := os.Lstat(targetPath); err == nil {
				if err := os.Remove(targetPath); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to remove existing file/link at %s: %v\n", targetPath, err)
					extractionErr = fmt.Errorf("failed to remove existing file/link at %s: %w", targetPath, err)
					break
				}
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create symlink %s -> %s: %v\n", targetPath, header.Linkname, err)
				extractionErr = fmt.Errorf("failed to create symlink %s -> %s: %w", targetPath, header.Linkname, err)
				break
			}
		default:
			// Log skipped types for debugging
		}
	}

	// Wait for all workers to finish
	wg.Wait()
	
	// Check for errors from workers
	close(errChan)
	for err := range errChan {
		if extractionErr == nil {
			extractionErr = err
		}
	}

	// Call with final progress
	if progressCb != nil {
		progressCb(1.0)
	}

	return extractionErr
}

// TODO: Add extractZip function if needed for other OS

// saveVersionMetadata saves the build info as version.json inside the extracted directory.
func saveVersionMetadata(build model.BlenderBuild, extractedDir string) error {
	metaPath := filepath.Join(extractedDir, versionMetaFilename)
	
	// Ensure the build date is set to current time if it's missing or zero
	if build.BuildDate.Time().IsZero() {
		build.BuildDate = model.Timestamp(time.Now())
	}
	
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
	
	// Create a temporary download directory inside the download base dir
	downloadTempDir := filepath.Join(downloadBaseDir, ".downloading")
	if err := os.MkdirAll(downloadTempDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create download temp dir: %w", err)
	}
	
	downloadPath := filepath.Join(downloadTempDir, downloadFileName)

	if err := downloadFile(build.DownloadURL, downloadPath, progressCb); err != nil {
		// Attempt cleanup even if download fails
		os.Remove(downloadPath)
		return "", fmt.Errorf("download failed: %w", err)
	}

	// 2. Determine Extraction Directory
	// For tar.xz files, extract the directory name from the archive
	baseNameWithoutExt := strings.TrimSuffix(strings.TrimSuffix(downloadFileName, ".tar.xz"), ".zip")
	extractedDirName := baseNameWithoutExt
	extractedPath := filepath.Join(downloadBaseDir, extractedDirName)

	// If directory already exists, move it to .oldbuild directory instead of deleting
	if _, err := os.Stat(extractedPath); err == nil {
		// Create .oldbuild parent directory if it doesn't exist
		oldBuildsDir := filepath.Join(downloadBaseDir, ".oldbuilds")
		if err := os.MkdirAll(oldBuildsDir, 0750); err != nil {
			return "", fmt.Errorf("failed to create .oldbuilds directory: %w", err)
		}
		
		// Create timestamped backup directory name
		timestamp := time.Now().Format("20060102_150405")
		oldBuildName := fmt.Sprintf("%s_%s", extractedDirName, timestamp)
		oldBuildPath := filepath.Join(oldBuildsDir, oldBuildName)
		
		// Move the old directory to .oldbuilds instead of removing it
		if err := os.Rename(extractedPath, oldBuildPath); err != nil {
			// If we can't move it (perhaps across filesystems), try to remove it as fallback
			fmt.Fprintf(os.Stderr, "Warning: couldn't move old build to backup dir: %v\n", err)
			fmt.Fprintf(os.Stderr, "Attempting to remove it instead...\n")
			if err := os.RemoveAll(extractedPath); err != nil {
				return "", fmt.Errorf("failed to remove existing target directory %s: %w", extractedPath, err)
			}
		}
	}

	// 3. Extract
	var extractErr error
	switch {
	case strings.HasSuffix(build.FileName, ".tar.xz"):
		// Create an extraction progress callback that adapts to our download progress callback format
		extractProgressCb := func(estimatedProgress float64) {
			if progressCb != nil {
				// We're using a fixed "extraction size" of 100MB to indicate extraction progress
				// The actual size doesn't matter, we just need something to show progress
				const extractionVirtualSize int64 = 100 * 1024 * 1024
				virtualProgress := int64(estimatedProgress * float64(extractionVirtualSize))
				progressCb(virtualProgress, extractionVirtualSize)
			}
		}
		
		extractErr = extractTarXz(downloadPath, downloadBaseDir, extractProgressCb)
	case strings.HasSuffix(build.FileName, ".zip"):
		extractErr = fmt.Errorf("zip extraction not yet implemented")
	default:
		extractErr = fmt.Errorf("unsupported archive type: %s", build.FileExtension)
	}

	// 4. Delete downloaded archive
	if err := os.Remove(downloadPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to delete archive %s: %v\n", downloadPath, err)
	}
	
	// Try to remove the temp download directory but don't error if it fails
	os.RemoveAll(downloadTempDir)

	if extractErr != nil {
		fmt.Fprintf(os.Stderr, "Extraction failed: %v\n", extractErr)
		// Clean up partial extraction if it exists
		if _, err := os.Stat(extractedPath); err == nil {
			os.RemoveAll(extractedPath)
		}
		return "", fmt.Errorf("extraction failed: %w", extractErr)
	}

	// 5. Save Metadata
	build.Status = "Local" // Set status to Local before saving
	if err := saveVersionMetadata(build, extractedPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save version metadata: %v\n", err)
	}

	return extractedPath, nil
}
