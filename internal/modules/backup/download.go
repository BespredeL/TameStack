package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tamestack/internal/ssh"
)

// DownloadProgressReporter receives progress updates during SFTP download.
type DownloadProgressReporter func(transferred, total int64)

// DownloadArchiveFromRemote streams an archive from the remote server via SFTP to a local file.
func DownloadArchiveFromRemote(ctx context.Context, client *ssh.SSHClient, remotePath, localDestPath string, progress DownloadProgressReporter) (string, error) {
	escapedRemote := ssh.ShellEscape(remotePath)

	_, _, _, _ = client.NewCommand(fmt.Sprintf("chmod 644 %s", escapedRemote)).WithSudo().Exec(ctx)

	sizeCmd := fmt.Sprintf("stat -c %%s %s 2>/dev/null", escapedRemote)
	outSize, _, _, err := client.NewCommand(sizeCmd).WithSudo().Exec(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to stat remote archive: %w", err)
	}
	totalBytes, _ := strconv.ParseInt(strings.TrimSpace(string(outSize)), 10, 64)
	if totalBytes <= 0 {
		return "", fmt.Errorf("remote archive is empty or missing: %s", remotePath)
	}

	if err := os.MkdirAll(filepath.Dir(localDestPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create local directory: %w", err)
	}

	partPath := localDestPath + ".part"
	outFile, err := os.Create(partPath)
	if err != nil {
		return "", fmt.Errorf("failed to create local file: %w", err)
	}

	hasher := sha256.New()
	multiWriter := io.MultiWriter(outFile, hasher)

	sftpClient, err := client.SFTP()
	if err != nil {
		outFile.Close()
		os.Remove(partPath)
		return "", fmt.Errorf("failed to start sftp subsystem: %w", err)
	}

	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		outFile.Close()
		os.Remove(partPath)
		return "", fmt.Errorf("failed to open remote file via SFTP: %w", err)
	}
	defer remoteFile.Close()

	buf := make([]byte, 64*1024)
	var transferred int64

	for {
		if ctx.Err() != nil {
			outFile.Close()
			os.Remove(partPath)
			return "", ctx.Err()
		}

		n, rErr := remoteFile.Read(buf)
		if n > 0 {
			if _, wErr := multiWriter.Write(buf[:n]); wErr != nil {
				outFile.Close()
				os.Remove(partPath)
				return "", fmt.Errorf("failed writing to local file: %w", wErr)
			}
			transferred += int64(n)
			if progress != nil {
				progress(transferred, totalBytes)
			}
		}

		if rErr == io.EOF {
			break
		}
		if rErr != nil {
			outFile.Close()
			os.Remove(partPath)
			return "", fmt.Errorf("error reading from remote SFTP stream: %w", rErr)
		}
	}

	outFile.Close()

	if err := VerifyLocalArchiveIntegrity(partPath); err != nil {
		os.Remove(partPath)
		return "", fmt.Errorf("downloaded archive integrity check failed (corrupted or truncated): %w", err)
	}

	if err := os.Rename(partPath, localDestPath); err != nil {
		return "", fmt.Errorf("failed to finalize downloaded file: %w", err)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	return checksum, nil
}

// VerifyLocalArchiveIntegrity tests whether the downloaded gzip/tar archive can be parsed cleanly.
func VerifyLocalArchiveIntegrity(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Check gzip header.
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("invalid gzip header: %w", err)
	}
	defer gz.Close()

	if strings.HasSuffix(filePath, ".tar.gz") || strings.HasSuffix(filePath, ".tgz") {
		tr := tar.NewReader(gz)
		readCount := 0
		for {
			_, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("corrupted tar archive stream: %w", err)
			}
			readCount++
			if readCount >= 5 {
				break
			}
		}
	} else {
		// Read first 4KB to confirm gzip stream isn't broken.
		buf := make([]byte, 4096)
		_, err = gz.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("corrupted gzip stream: %w", err)
		}
	}

	return nil
}
