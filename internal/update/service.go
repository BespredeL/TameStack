// Package update implements release checking against the project's GitHub
// feed and applying a self-update of the desktop binary on Windows.
//
// Language policy: errors that reach the user through the UI are Russian;
// internal diagnostics are English.
package update

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultRepo is the public repository publishing the release feed.
	DefaultRepo = "BespredeL/TameStack"

	apiBase   = "https://api.github.com"
	userAgent = "TameStack-Updater"

	// checkTimeout bounds metadata requests; the download itself is bounded
	// by the caller context so a slow link can abort explicitly.
	checkTimeout = 20 * time.Second
)

// Asset is one file attached to a GitHub release.
type Asset struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

// Release mirrors the fields of the GitHub releases/latest payload.
type Release struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	HTMLURL     string  `json:"html_url"`
	PublishedAt string  `json:"published_at"`
	Assets      []Asset `json:"assets"`
}

// CheckResult is what the UI needs to decide between "up to date" and
// "update available". JSON tags matter: the struct crosses the Wails bridge.
type CheckResult struct {
	CurrentVersion string   `json:"current_version"`
	LatestVersion  string   `json:"latest_version"`
	Available      bool     `json:"available"`
	AssetFound     bool     `json:"asset_found"`
	Release        *Release `json:"release"`
}

// Progress reports one stage of an installation run.
type Progress struct {
	Phase      string `json:"phase"` // downloading | verifying | installing
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
}

// Service checks the release feed and installs updates over the running
// binary. Fields with lowercase names are test seams.
type Service struct {
	repo       string
	current    string
	baseURL    string
	webBaseURL string // defaults to "https://github.com"
	client     *http.Client

	target    string // binary to replace; defaults to os.Executable()
	elevateFn func(sourceExe, targetExe string, waitPID int) error

	mu              sync.Mutex
	cachedResult    *CheckResult
	cachedAt        time.Time
	cachedETag      string
	elevatedPending bool
}

func newHTTPClient() *http.Client {
	if transport, ok := http.DefaultTransport.(*http.Transport); ok {
		cloned := transport.Clone()
		cloned.ResponseHeaderTimeout = 30 * time.Second
		return &http.Client{Transport: cloned}
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func NewService(repo, currentVersion string) *Service {
	return &Service{
		repo:       repo,
		current:    currentVersion,
		baseURL:    apiBase,
		webBaseURL: "https://github.com",
		client:     newHTTPClient(),
	}
}

func (s *Service) newRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

// Check fetches the latest release and compares it with the running version.
// It caches results for 2 minutes and uses ETag conditional requests as well
// as web-redirect fallback to avoid GitHub API rate limits.
func (s *Service) Check(ctx context.Context) (*CheckResult, error) {
	s.mu.Lock()
	if s.cachedResult != nil && time.Since(s.cachedAt) < 2*time.Minute {
		res := s.cachedResult
		s.mu.Unlock()
		return res, nil
	}
	etag := s.cachedETag
	s.mu.Unlock()

	url := fmt.Sprintf("%s/repos/%s/releases/latest", s.baseURL, s.repo)
	reqCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	req, err := s.newRequest(reqCtx, url)
	if err != nil {
		return nil, err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.mu.Lock()
		if s.cachedResult != nil {
			cached := s.cachedResult
			s.mu.Unlock()
			return cached, nil
		}
		s.mu.Unlock()
		return nil, fmt.Errorf("не удалось связаться с GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		s.mu.Lock()
		if s.cachedResult != nil {
			s.cachedAt = time.Now()
			res := s.cachedResult
			s.mu.Unlock()
			return res, nil
		}
		s.mu.Unlock()
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		s.mu.Lock()
		if s.cachedResult != nil {
			res := s.cachedResult
			s.mu.Unlock()
			return res, nil
		}
		s.mu.Unlock()

		// Fallback to web redirect (releases/latest) when API rate limit is exceeded
		res, fbErr := s.checkViaWebRedirect(ctx)
		if fbErr == nil {
			s.mu.Lock()
			s.cachedResult = res
			s.cachedAt = time.Now()
			s.mu.Unlock()
			return res, nil
		}

		return nil, fmt.Errorf("превышен лимит обращений к GitHub, попробуйте позже")
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("релизы не найдены (репозиторий %s недоступен?)", s.repo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub вернул статус %d", resp.StatusCode)
	}

	newETag := resp.Header.Get("ETag")

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("некорректный ответ GitHub: %w", err)
	}

	latest := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	cmp, ok := compareVersions(latest, s.current)
	if !ok {
		return nil, fmt.Errorf("некорректный номер версии релиза: %q", release.TagName)
	}

	result := &CheckResult{
		CurrentVersion: s.current,
		LatestVersion:  latest,
		Available:      cmp > 0,
		Release:        &release,
	}
	if s.pickAsset(&release) != nil {
		result.AssetFound = true
	}

	s.mu.Lock()
	s.cachedResult = result
	s.cachedAt = time.Now()
	if newETag != "" {
		s.cachedETag = newETag
	}
	s.mu.Unlock()

	return result, nil
}

// checkViaWebRedirect resolves the latest tag via a public web redirect without hitting API quotas.
func (s *Service) checkViaWebRedirect(ctx context.Context) (*CheckResult, error) {
	webBase := s.webBaseURL
	if webBase == "" {
		webBase = "https://github.com"
	}
	webURL := fmt.Sprintf("%s/%s/releases/latest", webBase, s.repo)
	reqCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, webURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: checkTimeout,
	}

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently && resp.StatusCode != http.StatusSeeOther {
		return nil, fmt.Errorf("unexpected status from release page: %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		return nil, fmt.Errorf("missing Location header in redirect")
	}

	parts := strings.Split(strings.TrimRight(loc, "/"), "/")
	tag := parts[len(parts)-1]
	if tag == "" || tag == "latest" {
		return nil, fmt.Errorf("could not extract release tag from location %q", loc)
	}

	latest := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	cmp, ok := compareVersions(latest, s.current)
	if !ok {
		return nil, fmt.Errorf("некорректный номер версии релиза: %q", tag)
	}

	release := &Release{
		TagName: tag,
		Name:    "TameStack " + tag,
		HTMLURL: loc,
		Assets:  s.buildSyntheticAssets(tag, webBase),
	}

	result := &CheckResult{
		CurrentVersion: s.current,
		LatestVersion:  latest,
		Available:      cmp > 0,
		Release:        release,
	}
	if s.pickAsset(release) != nil {
		result.AssetFound = true
	}
	return result, nil
}

func (s *Service) buildSyntheticAssets(tag, webBase string) []Asset {
	candidates := candidateAssets(runtime.GOOS, runtime.GOARCH)
	assets := make([]Asset, 0, len(candidates))
	for _, name := range candidates {
		assets = append(assets, Asset{
			ID:          0,
			Name:        name,
			DownloadURL: fmt.Sprintf("%s/%s/releases/download/%s/%s", webBase, s.repo, tag, name),
		})
	}
	return assets
}

// candidateAssets returns the release asset names carrying the binary for the
// current OS, ordered by preference. Direct executables take precedence over
// legacy archives.
func candidateAssets(goos, goarch string) []string {
	switch goos {
	case "windows":
		return []string{
			"TameStack-windows-amd64.exe",
			"TameStack-windows-amd64-portable.exe",
			"TameStack.exe",
			"TameStack-windows-amd64.zip",
		}
	case "linux":
		return []string{
			"TameStack-linux-amd64",
			"TameStack",
			"TameStack-linux-amd64.tar.gz",
		}
	case "darwin":
		return []string{
			"TameStack-darwin-universal.zip",
		}
	}
	return nil
}

// assetNameForPlatform is the primary release asset name carrying the binary for
// the current OS. It mirrors the asset_name matrix in .github/workflows/build.yml.
func assetNameForPlatform(goos, goarch string) string {
	candidates := candidateAssets(goos, goarch)
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func (s *Service) pickAsset(release *Release) *Asset {
	candidates := candidateAssets(runtime.GOOS, runtime.GOARCH)
	for _, want := range candidates {
		for i := range release.Assets {
			if release.Assets[i].Name == want {
				return &release.Assets[i]
			}
		}
	}
	return nil
}

// CleanupOldBinaries removes the <exe>.old copies left behind by previous
// self-updates. A running process cannot delete its own loaded image, so the
// next start does it. Best-effort by design.
func CleanupOldBinaries() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	removeWithRetry(exe + ".old")
	leftovers, _ := filepath.Glob(exe + ".old-*")
	for _, p := range leftovers {
		removeWithRetry(p)
	}
}

func removeWithRetry(path string) {
	for attempt := 0; attempt < 3; attempt++ {
		if err := os.Remove(path); err == nil || os.IsNotExist(err) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (s *Service) resolveTarget() (string, error) {
	if s.target != "" {
		return s.target, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("не удалось определить путь к запущенному приложению: %w", err)
	}
	return exe, nil
}

// IsElevatedPending reports whether the last Install() call handed off the
// binary swap to an elevated helper process via UAC.
func (s *Service) IsElevatedPending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.elevatedPending
}

// Install downloads the platform asset, verifies its GitHub-provided SHA-256
// digest, extracts the binary and swaps it under the running executable.
// When the installation directory is write-protected (e.g. Program Files),
// the verified binary is staged in %TEMP% and applied via UAC elevation.
// onProgress may be nil. The caller decides when to restart the app.
func (s *Service) Install(ctx context.Context, onProgress func(Progress)) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("автообновление доступно только в Windows; скачайте сборку вручную")
	}

	s.mu.Lock()
	s.elevatedPending = false
	s.mu.Unlock()

	target, err := s.resolveTarget()
	if err != nil {
		return err
	}

	result, err := s.Check(ctx)
	if err != nil {
		return err
	}
	if !result.Available {
		return fmt.Errorf("установлена актуальная версия %s", result.CurrentVersion)
	}
	asset := s.pickAsset(result.Release)
	if asset == nil {
		return fmt.Errorf("в релизе %s нет архива для этой платформы", result.LatestVersion)
	}

	writable := isDirWritable(filepath.Dir(target))
	var staging string
	if writable {
		staging, err = os.MkdirTemp(filepath.Dir(target), ".shupdate-")
	} else {
		staging, err = os.MkdirTemp("", "tamestack-update-*")
	}
	if err != nil {
		return fmt.Errorf("не удалось подготовить каталог обновления: %w", err)
	}
	cleanupStaging := true
	defer func() {
		if cleanupStaging {
			_ = os.RemoveAll(staging)
		}
	}()

	report := func(p Progress) {
		if onProgress != nil {
			onProgress(p)
		}
	}

	archivePath := filepath.Join(staging, asset.Name)
	if err := s.download(ctx, asset.DownloadURL, archivePath, func(downloaded, total int64) {
		report(Progress{Phase: "downloading", Downloaded: downloaded, Total: total})
	}); err != nil {
		return err
	}

	report(Progress{Phase: "verifying", Downloaded: 0, Total: 0})
	if err := s.verifyDigest(ctx, asset, archivePath); err != nil {
		os.Remove(archivePath)
		return err
	}

	binaryPath := filepath.Join(staging, filepath.Base(target))
	if strings.HasSuffix(strings.ToLower(asset.Name), ".zip") {
		if err := extractBinary(archivePath, filepath.Base(target), binaryPath); err != nil {
			return err
		}
	} else {
		if err := moveFile(archivePath, binaryPath); err != nil {
			return fmt.Errorf("не удалось подготовить файл обновления: %w", err)
		}
	}

	report(Progress{Phase: "installing", Downloaded: 0, Total: 0})
	if writable {
		if err := swapBinary(binaryPath, target); err != nil {
			return err
		}
		return nil
	}

	elevate := s.elevateFn
	if elevate == nil {
		elevate = SpawnElevatedApply
	}
	if err := elevate(binaryPath, target, os.Getpid()); err != nil {
		return fmt.Errorf("для обновления в защищённом каталоге требуются права администратора: %w", err)
	}
	cleanupStaging = false
	s.mu.Lock()
	s.elevatedPending = true
	s.mu.Unlock()
	return nil
}

func (s *Service) download(ctx context.Context, url, destPath string, onBytes func(done, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("не удалось скачать обновление: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("скачивание не удалось: GitHub вернул статус %d", resp.StatusCode)
	}

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("не удалось создать файл загрузки: %w", err)
	}

	total := resp.ContentLength
	var done int64
	buf := make([]byte, 256*1024)
	for {
		if err := ctx.Err(); err != nil {
			out.Close()
			os.Remove(destPath)
			return err
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				out.Close()
				os.Remove(destPath)
				return fmt.Errorf("запись файла загрузки прервана: %w", writeErr)
			}
			done += int64(n)
			onBytes(done, total)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			os.Remove(destPath)
			return fmt.Errorf("скачивание прервано: %w", readErr)
		}
	}
	if err := out.Close(); err != nil {
		os.Remove(destPath)
		return err
	}
	return nil
}

// verifyDigest checks the archive against the SHA-256 digest GitHub publishes
// for the asset. A missing digest is a hard failure: an unverified binary is
// never swapped in.
func (s *Service) verifyDigest(ctx context.Context, asset *Asset, archivePath string) error {
	if asset.ID <= 0 {
		return fmt.Errorf("контрольная сумма недоступна через GitHub API; скачайте обновление по кнопке «Открыть страницу релиза»")
	}
	url := fmt.Sprintf("%s/repos/%s/releases/assets/%d", s.baseURL, s.repo, asset.ID)
	reqCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	req, err := s.newRequest(reqCtx, url)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("не удалось получить контрольную сумму: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub вернул статус %d для контрольной суммы", resp.StatusCode)
	}

	var meta struct {
		Digest string `json:"digest"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return fmt.Errorf("некорректный ответ с контрольной суммой: %w", err)
	}

	var rawHex string
	switch {
	case strings.HasPrefix(meta.Digest, "sha256:"):
		rawHex = strings.TrimPrefix(meta.Digest, "sha256:")
	case strings.HasPrefix(meta.Digest, "sha256="):
		rawHex = strings.TrimPrefix(meta.Digest, "sha256=")
	default:
		return fmt.Errorf("GitHub не опубликовал SHA-256 для этого релиза — обновление заблокировано")
	}
	want, err := hex.DecodeString(rawHex)
	if err != nil || len(want) != sha256.Size {
		return fmt.Errorf("некорректная контрольная сумма от GitHub")
	}

	got, err := fileSHA256(archivePath)
	if err != nil {
		return err
	}
	if !sha256Equal(want, got) {
		return fmt.Errorf("контрольная сумма архива не совпала — обновление отменено")
	}
	return nil
}

func isDirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".permcheck-")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

func fileSHA256(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать скачанный архив: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("подсчёт контрольной суммы прерван: %w", err)
	}
	return h.Sum(nil), nil
}

func sha256Equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// extractBinary unpacks only the requested base name from the archive. Entry
// names are reduced to their base component, so traversal payloads in a
// malicious archive cannot escape staging.
func extractBinary(archivePath, baseName, destPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("не удалось открыть архив обновления: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		// Only a root-level entry with the exact binary name is accepted;
		// nested or traversal-named copies are skipped entirely.
		if f.FileInfo().IsDir() || !strings.EqualFold(f.Name, baseName) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("не удалось прочитать %s из архива: %w", baseName, err)
		}
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0755)
		if err != nil {
			rc.Close()
			return fmt.Errorf("не удалось записать новый бинарник: %w", err)
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			os.Remove(destPath)
			return fmt.Errorf("распаковка прервана: %w", copyErr)
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	}
	return fmt.Errorf("в архиве не найден файл %s", baseName)
}

// swapBinary moves the running executable aside as <exe>.old and puts the new
// binary in its place, restoring the old one when the second move fails.
// Windows forbids deleting a loaded image but allows renaming it.
func swapBinary(newPath, targetPath string) error {
	oldPath, err := graveyardPath(targetPath)
	if err != nil {
		return fmt.Errorf("не удалось подготовить место для обновления: %w", err)
	}
	if err := os.Rename(targetPath, oldPath); err != nil {
		return fmt.Errorf("не удалось подготовить место для обновления: %w", err)
	}
	if err := moveFile(newPath, targetPath); err != nil {
		if restoreErr := os.Rename(oldPath, targetPath); restoreErr != nil {
			return fmt.Errorf("установка не удалась (%v) и старая версия не восстановлена: %w", err, restoreErr)
		}
		return fmt.Errorf("не удалось установить новый файл программы: %w", err)
	}
	return nil
}

// graveyardPath is where the running executable gets renamed. The usual
// <exe>.old is preferred, but when it is still locked by a previous update
// a unique sibling name is used instead.
func graveyardPath(targetPath string) (string, error) {
	oldPath := targetPath + ".old"
	if err := os.Remove(oldPath); err == nil || os.IsNotExist(err) {
		return oldPath, nil
	}
	f, err := os.CreateTemp(filepath.Dir(targetPath), filepath.Base(targetPath)+".old-")
	if err != nil {
		return "", err
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return name, nil
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		in.Close()
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeOut := out.Close()
	in.Close()
	if copyErr != nil {
		os.Remove(dst)
		return copyErr
	}
	if closeOut != nil {
		return closeOut
	}
	os.Remove(src)
	return nil
}
