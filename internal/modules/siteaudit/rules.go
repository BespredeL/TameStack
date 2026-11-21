package siteaudit

import (
	"path"
	"strings"

	"tamestack/internal/crypto"
)

// FileEntry is one file inside a site document root.
type FileEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Mode   string `json:"mode"`
	Owner  string `json:"owner"`
	Mtime  string `json:"mtime"`
	SHA256 string `json:"sha256"`
}

// Finding is a rule match. It is not a verdict that the file is malware.
type Finding struct {
	Path    string `json:"path"`
	RuleID  string `json:"rule_id"`
	Summary string `json:"summary"`
}

// Diff describes how a snapshot differs from a baseline.
type Diff struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

var scriptExtensions = map[string]bool{
	".php": true, ".phtml": true, ".php5": true, ".phar": true, ".pht": true,
}

var uploadDirs = []string{"uploads", "upload", "wp-content/uploads", "storage/app/public"}

// ParseManifest reads tab-separated lines: path, size, mode, owner, mtime, sha256.
func ParseManifest(raw string) []FileEntry {
	var out []FileEntry
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 6 {
			continue
		}
		rel := path.Clean(strings.TrimPrefix(parts[0], "./"))
		if rel == "." || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") || strings.HasPrefix(rel, "/") {
			continue
		}
		var size int64
		for _, r := range parts[1] {
			if r < '0' || r > '9' {
				size = 0
				break
			}
			size = size*10 + int64(r-'0')
		}
		out = append(out, FileEntry{
			Path: rel, Size: size, Mode: parts[2], Owner: parts[3], Mtime: parts[4], SHA256: parts[5],
		})
	}
	return out
}

// CompareManifests returns paths that appeared, disappeared, or changed hash.
func CompareManifests(base, next []FileEntry) Diff {
	old := map[string]string{}
	for _, f := range base {
		old[f.Path] = f.SHA256
	}
	seen := map[string]bool{}
	var d Diff
	for _, f := range next {
		seen[f.Path] = true
		prev, ok := old[f.Path]
		if !ok {
			d.Added = append(d.Added, f.Path)
			continue
		}
		if prev != f.SHA256 {
			d.Changed = append(d.Changed, f.Path)
		}
	}
	for p := range old {
		if !seen[p] {
			d.Removed = append(d.Removed, p)
		}
	}
	return d
}

// EvaluateRules marks suspicious paths and short text excerpts.
func EvaluateRules(files []FileEntry, diff Diff, excerpts map[string]string) []Finding {
	interest := map[string]bool{}
	for _, p := range diff.Added {
		interest[p] = true
	}
	for _, p := range diff.Changed {
		interest[p] = true
	}
	var findings []Finding
	for _, f := range files {
		ext := strings.ToLower(path.Ext(f.Path))
		base := strings.ToLower(path.Base(f.Path))
		if inUpload(f.Path) && scriptExtensions[ext] {
			findings = append(findings, Finding{Path: f.Path, RuleID: "script-in-uploads", Summary: "Скрипт лежит в каталоге загрузок."})
		}
		if strings.Count(base, ".") >= 2 && scriptExtensions[path.Ext(base)] {
			findings = append(findings, Finding{Path: f.Path, RuleID: "double-extension", Summary: "У файла двойное расширение, последнее — скрипт."})
		}
		text := excerpts[f.Path]
		if text == "" || !interest[f.Path] {
			continue
		}
		low := strings.ToLower(crypto.ScrubSecrets(text))
		if strings.Contains(low, "eval(") && (strings.Contains(low, "base64_decode") || strings.Contains(low, "gzinflate") || strings.Contains(low, "str_rot13")) {
			findings = append(findings, Finding{Path: f.Path, RuleID: "encoded-eval", Summary: "В новом или изменённом файле eval сочетается с декодированием."})
		}
		if strings.Contains(low, "shell_exec") || strings.Contains(low, "passthru(") || strings.Contains(low, "system($_") || strings.Contains(low, "proc_open") {
			findings = append(findings, Finding{Path: f.Path, RuleID: "shell-from-request", Summary: "В новом или изменённом файле есть запуск команды оболочки."})
		}
	}
	return findings
}

func inUpload(p string) bool {
	low := strings.ToLower(p)
	for _, dir := range uploadDirs {
		if low == dir || strings.HasPrefix(low, dir+"/") || strings.Contains(low, "/"+dir+"/") {
			return true
		}
	}
	return false
}
