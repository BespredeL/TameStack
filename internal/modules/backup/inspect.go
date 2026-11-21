package backup

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"tamestack/internal/ssh"
)

var systemPrefixes = []string{
	"etc/",
	"/etc/",
	"bin/",
	"/bin/",
	"sbin/",
	"/sbin/",
	"usr/bin/",
	"/usr/bin/",
	"usr/sbin/",
	"/usr/sbin/",
	"usr/lib/",
	"/usr/lib/",
	"lib/",
	"/lib/",
	"lib64/",
	"/lib64/",
	"boot/",
	"/boot/",
	"root/",
	"/root/",
	"dev/",
	"/dev/",
	"proc/",
	"/proc/",
	"sys/",
	"/sys/",
}

// DetectDangerousPaths inspects a list of file paths from an archive and flags system critical paths.
func DetectDangerousPaths(entries []string) []string {
	var dangerous []string
	for _, entry := range entries {
		clean := strings.TrimSpace(entry)
		if clean == "" || clean == "." || clean == "./" {
			continue
		}
		// Path traversal check.
		if strings.Contains(clean, "..") {
			dangerous = append(dangerous, clean+" (path traversal)")
			continue
		}

		// Exact matches for critical files.
		trimmedLeading := strings.TrimPrefix(clean, "/")
		trimmedLeading = strings.TrimPrefix(trimmedLeading, "./")
		if trimmedLeading == "etc/passwd" || trimmedLeading == "etc/shadow" || trimmedLeading == "etc/sudoers" ||
			trimmedLeading == "etc/crontab" || trimmedLeading == "etc/fstab" {
			dangerous = append(dangerous, clean)
			continue
		}

		// Prefix checks
		for _, prefix := range systemPrefixes {
			if strings.HasPrefix(clean, prefix) || strings.HasPrefix(trimmedLeading, strings.TrimPrefix(prefix, "/")) {
				dangerous = append(dangerous, clean)
				break
			}
		}
	}
	return dangerous
}

// CheckArchiveIntegrity checks if the remote archive can be unpacked without corruption.
func CheckArchiveIntegrity(ctx context.Context, client *ssh.SSHClient, remotePath string) error {
	escaped := ssh.ShellEscape(remotePath)
	var cmd string
	if strings.HasSuffix(remotePath, ".tar.gz") || strings.HasSuffix(remotePath, ".tgz") {
		cmd = fmt.Sprintf("gzip -t %s && tar -tzf %s >/dev/null", escaped, escaped)
	} else if strings.HasSuffix(remotePath, ".sql.gz") || strings.HasSuffix(remotePath, ".gz") {
		cmd = fmt.Sprintf("gzip -t %s", escaped)
	} else {
		cmd = fmt.Sprintf("test -s %s", escaped)
	}

	stdout, stderr, code, err := client.NewCommand(cmd).WithSudo().Exec(ctx)
	if err != nil || code != 0 {
		return fmt.Errorf("archive integrity check failed: %s %s (code %d, err %v)", string(stdout), string(stderr), code, err)
	}
	return nil
}

// InspectArchive reads remote archive metadata and inspects contents for dangerous files.
func InspectArchive(ctx context.Context, client *ssh.SSHClient, remotePath string) (*ArchiveInspectionResult, error) {
	escaped := ssh.ShellEscape(remotePath)
	res := &ArchiveInspectionResult{
		FilePath:    remotePath,
		InspectedAt: time.Now(),
	}

	sizeOut, _, _, _ := client.NewCommand(fmt.Sprintf("stat -c %%s %s 2>/dev/null", escaped)).WithSudo().Exec(ctx)
	sizeBytes, _ := strconv.ParseInt(strings.TrimSpace(string(sizeOut)), 10, 64)
	res.SizeBytes = sizeBytes

	if strings.HasSuffix(remotePath, ".tar.gz") || strings.HasSuffix(remotePath, ".tgz") {
		res.ArchiveType = "tar.gz"
	} else if strings.HasSuffix(remotePath, ".sql.gz") {
		res.ArchiveType = "sql.gz"
		res.ContainsDBDump = true
		res.DBDumpName = path.Base(remotePath)
	} else {
		res.ArchiveType = "unknown"
	}

	if err := CheckArchiveIntegrity(ctx, client, remotePath); err != nil {
		res.IntegrityPassed = false
		res.ErrorMessage = err.Error()
		return res, nil
	}
	res.IntegrityPassed = true

	if res.ArchiveType == "tar.gz" {
		listCmd := fmt.Sprintf("tar -tzf %s | head -n 500", escaped)
		out, stderr, code, err := client.NewCommand(listCmd).WithSudo().Exec(ctx)
		if err != nil || code != 0 {
			res.ErrorMessage = fmt.Sprintf("failed to list archive contents: %s", string(stderr))
			return res, nil
		}

		rawLines := strings.Split(string(out), "\n")
		var entries []string
		for _, l := range rawLines {
			t := strings.TrimSpace(l)
			if t != "" {
				entries = append(entries, t)
				if strings.HasSuffix(t, ".sql") {
					res.ContainsDBDump = true
					res.DBDumpName = t
				}
			}
		}

		res.TotalEntries = len(entries)
		if len(entries) > 50 {
			res.SampleEntries = entries[:50]
		} else {
			res.SampleEntries = entries
		}

		// Detect system / dangerous paths.
		dangerous := DetectDangerousPaths(entries)
		if len(dangerous) > 0 {
			res.HasSystemPaths = true
			res.DangerousPaths = dangerous
		}

		// Suggest extraction root.
		if len(entries) > 0 {
			first := entries[0]
			if strings.HasPrefix(first, "var/www/") || strings.HasPrefix(first, "./var/www/") {
				parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(first, "./"), "var/www/"), "/")
				if len(parts) > 0 && parts[0] != "" {
					res.SuggestedRoot = "/var/www/" + parts[0]
				}
			}
		}
	}

	return res, nil
}
