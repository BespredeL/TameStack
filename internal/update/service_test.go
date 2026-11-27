package update

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
		ok   bool
	}{
		{"1.0.0", "1.0.0", 0, true},
		{"v1.0.0", "1.0.0", 0, true},
		{"1.2.3", "1.2.4", -1, true},
		{"1.10.0", "1.9.9", 1, true},
		{"2.0.0", "1.99.99", 1, true},
		{"1.2", "1.2.0", 0, true},
		{"1.2.3-rc1", "1.2.3", 0, true},
		{"1.2.3+build", "1.2.4", -1, true},
		{"", "1.0.0", 0, false},
		{"x.y.z", "1.0.0", 0, false},
		{"1.2.3.4", "1.0.0", 0, false},
	}
	for _, c := range cases {
		got, ok := compareVersions(c.a, c.b)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("compareVersions(%q,%q) = %d,%v; want %d,%v", c.a, c.b, got, ok, c.want, c.ok)
		}
	}
}

func platformAsset() string {
	name := assetNameForPlatform(runtime.GOOS, runtime.GOARCH)
	if name == "" {
		name = "TameStack-test-asset.zip"
	}
	return name
}

type assetFixture struct {
	id   int
	name string
	path string // served under the httptest server
}

// releaseJSON builds a GitHub releases payload; download URLs point back at
// the serving request so the same helper works behind any httptest listener.
func releaseJSON(t *testing.T, r *http.Request, tag string, assets []assetFixture) []byte {
	t.Helper()
	items := make([]map[string]any, 0, len(assets))
	for _, a := range assets {
		items = append(items, map[string]any{
			"id":                   a.id,
			"name":                 a.name,
			"browser_download_url": fmt.Sprintf("http://%s%s", r.Host, a.path),
			"size":                 1,
		})
	}
	body, err := json.Marshal(map[string]any{
		"tag_name": tag,
		"name":     "Release " + tag,
		"body":     "notes",
		"html_url": "https://github.com/test/repo/releases/tag/" + tag,
		"assets":   items,
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestCheckDetectsUpdate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-GitHub-Api-Version") == "" || r.Header.Get("User-Agent") == "" {
			t.Errorf("missing GitHub API headers")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(releaseJSON(t, r, "v9.9.9", []assetFixture{{42, platformAsset(), "/download/42"}}))
	})
	s := fixtureService(t, mux, "1.0.0")

	res, err := s.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Available || !res.AssetFound {
		t.Fatalf("expected available+asset, got %+v", res)
	}
	if res.LatestVersion != "9.9.9" || res.CurrentVersion != "1.0.0" {
		t.Fatalf("versions: %+v", res)
	}
}

func TestCheckUpToDate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Write(releaseJSON(t, r, "v1.0.0", []assetFixture{{42, platformAsset(), "/download/42"}}))
	})
	s := fixtureService(t, mux, "1.0.0")

	res, err := s.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Available {
		t.Fatalf("expected no update, got %+v", res)
	}
}

func TestCheckOlderReleaseIgnored(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Write(releaseJSON(t, r, "v0.9.0", []assetFixture{{42, platformAsset(), "/download/42"}}))
	})
	s := fixtureService(t, mux, "1.0.0")

	res, err := s.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Available {
		t.Fatal("older release must not offer an update")
	}
}

func TestCheckMissingPlatformAsset(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Write(releaseJSON(t, r, "v9.9.9", []assetFixture{{42, "SomethingElse.zip", "/download/42"}}))
	})
	s := fixtureService(t, mux, "1.0.0")

	res, err := s.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Available || res.AssetFound {
		t.Fatalf("expected available without asset, got %+v", res)
	}
}

func TestCheckRateLimitedFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	s := fixtureService(t, mux, "1.0.0")

	_, err := s.Check(context.Background())
	if err == nil {
		t.Fatal("expected error on 403")
	}
	if !strings.Contains(err.Error(), "лимит") {
		t.Fatalf("error should mention the GitHub limit: %v", err)
	}
}

func TestCheckInvalidTagFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Write(releaseJSON(t, r, "not-a-version", []assetFixture{{42, platformAsset(), "/download/42"}}))
	})
	s := fixtureService(t, mux, "1.0.0")

	_, err := s.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for non-semver tag")
	}
}

func fixtureService(t *testing.T, mux *http.ServeMux, current string) *Service {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := NewService("test/repo", current)
	s.baseURL = srv.URL
	s.client = srv.Client()
	return s
}

func makeZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(content))
	}
	zw.Close()
	return buf.Bytes()
}

func TestExtractBinaryIgnoresTraversalEntries(t *testing.T) {
	dir := t.TempDir()
	archive := makeZip(t, map[string]string{
		"../../evil.txt":       "evil",
		"nested/TameStack.exe": "nested-new",
		"TameStack.exe":        "good-new",
	})
	archivePath := filepath.Join(dir, "rel.zip")
	if err := os.WriteFile(archivePath, archive, 0600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "TameStack.exe")
	if err := extractBinary(archivePath, "TameStack.exe", dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "good-new" {
		t.Fatalf("bad extraction: %q %v", got, err)
	}
	parent := filepath.Dir(dir)
	if _, err := os.Stat(filepath.Join(parent, "evil.txt")); err == nil {
		t.Fatal("traversal entry escaped staging")
	}
}

func TestExtractBinaryMissingEntry(t *testing.T) {
	dir := t.TempDir()
	archive := makeZip(t, map[string]string{"Readme.txt": "hi"})
	archivePath := filepath.Join(dir, "rel.zip")
	if err := os.WriteFile(archivePath, archive, 0600); err != nil {
		t.Fatal(err)
	}
	if err := extractBinary(archivePath, "TameStack.exe", filepath.Join(dir, "x")); err == nil {
		t.Fatal("expected error when the binary is not in the archive")
	}
}

func TestSwapBinaryKeepsOldCopy(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "TameStack.exe")
	if err := os.WriteFile(target, []byte("OLD"), 0755); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(dir, "new.exe")
	if err := os.WriteFile(fresh, []byte("NEW"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := swapBinary(fresh, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "NEW" {
		t.Fatalf("target not swapped: %q %v", got, err)
	}
	old, err := os.ReadFile(target + ".old")
	if err != nil || string(old) != "OLD" {
		t.Fatalf("old copy missing: %q %v", old, err)
	}
}

func TestSwapBinaryRestoreOnFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "TameStack.exe")
	if err := os.WriteFile(target, []byte("OLD"), 0755); err != nil {
		t.Fatal(err)
	}
	// Nonexistent source makes the second move fail; target must be restored.
	if err := swapBinary(filepath.Join(dir, "missing.exe"), target); err == nil {
		t.Fatal("expected swap to fail")
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "OLD" {
		t.Fatalf("target not restored: %q %v", got, err)
	}
}

func installerFixture(t *testing.T, payload []byte, digest func() string) (*Service, string) {
	return installerFixtureWithAsset(t, platformAsset(), payload, digest)
}

func installerFixtureWithAsset(t *testing.T, assetName string, payload []byte, digest func() string) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "TameStack.exe")
	if err := os.WriteFile(target, []byte("OLD-EXE"), 0755); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Write(releaseJSON(t, r, "v9.9.9", []assetFixture{{42, assetName, "/download/42"}}))
	})
	mux.HandleFunc("/repos/test/repo/releases/assets/42", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"digest": digest()})
	})
	mux.HandleFunc("/download/42", func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := NewService("test/repo", "1.0.0")
	s.baseURL = srv.URL
	s.client = srv.Client()
	s.target = target
	return s, target
}

func sha256DigestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256=" + hex.EncodeToString(sum[:])
}

func TestInstallHappyPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("self-update swap is Windows-only")
	}
	exeBytes := []byte("NEW-EXE")
	s, target := installerFixture(t, exeBytes, func() string { return sha256DigestOf(exeBytes) })

	var phases []string
	if err := s.Install(context.Background(), func(p Progress) {
		if p.Phase != "" && (len(phases) == 0 || phases[len(phases)-1] != p.Phase) {
			phases = append(phases, p.Phase)
		}
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "NEW-EXE" {
		t.Fatalf("binary not updated: %q %v", got, err)
	}
	old, err := os.ReadFile(target + ".old")
	if err != nil || string(old) != "OLD-EXE" {
		t.Fatalf("old binary missing: %q %v", old, err)
	}
	want := []string{"downloading", "verifying", "installing"}
	if strings.Join(phases, ",") != strings.Join(want, ",") {
		t.Fatalf("progress phases = %v, want %v", phases, want)
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if name != "TameStack.exe" && name != "TameStack.exe.old" {
			t.Fatalf("staging leftover in exe dir: %s", name)
		}
	}
}

func TestInstallZipArchiveHappyPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("self-update swap is Windows-only")
	}
	zipBytes := makeZip(t, map[string]string{"TameStack.exe": "NEW-ZIP-EXE"})
	s, target := installerFixtureWithAsset(t, "TameStack-windows-amd64.zip", zipBytes, func() string { return sha256DigestOf(zipBytes) })

	if err := s.Install(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "NEW-ZIP-EXE" {
		t.Fatalf("binary not updated from zip: %q %v", got, err)
	}
	old, err := os.ReadFile(target + ".old")
	if err != nil || string(old) != "OLD-EXE" {
		t.Fatalf("old binary missing: %q %v", old, err)
	}
}

func TestInstallColonDigestHappyPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("self-update swap is Windows-only")
	}
	exeBytes := []byte("NEW-EXE-COLON")
	s, target := installerFixture(t, exeBytes, func() string {
		sum := sha256.Sum256(exeBytes)
		return "sha256:" + hex.EncodeToString(sum[:])
	})

	if err := s.Install(context.Background(), nil); err != nil {
		t.Fatalf("Install failed with sha256: prefix: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "NEW-EXE-COLON" {
		t.Fatalf("binary not updated: %q %v", got, err)
	}
}

func TestInstallDigestMismatchAborts(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("self-update swap is Windows-only")
	}
	exeBytes := []byte("NEW-EXE")
	s, target := installerFixture(t, exeBytes, func() string {
		return "sha256=" + hex.EncodeToString(make([]byte, 32))
	})

	if err := s.Install(context.Background(), nil); err == nil {
		t.Fatal("expected digest mismatch error")
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "OLD-EXE" {
		t.Fatalf("target modified despite mismatch: %q %v", got, err)
	}
	if _, err := os.Stat(target + ".old"); err == nil {
		t.Fatal("swap must not have started")
	}
}

func TestInstallMissingDigestAborts(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("self-update swap is Windows-only")
	}
	exeBytes := []byte("NEW-EXE")
	s, target := installerFixture(t, exeBytes, func() string { return "" })

	if err := s.Install(context.Background(), nil); err == nil {
		t.Fatal("expected fail-closed error when GitHub publishes no digest")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "OLD-EXE" {
		t.Fatalf("target modified: %q", got)
	}
}

func TestCheckUsesCache(t *testing.T) {
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write(releaseJSON(t, r, "v2.0.0", []assetFixture{{42, platformAsset(), "/download/42"}}))
	})
	s := fixtureService(t, mux, "1.0.0")

	res1, err := s.Check(context.Background())
	if err != nil || !res1.Available {
		t.Fatalf("first check failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}

	// Second check should hit the in-memory cache without calling backend
	res2, err := s.Check(context.Background())
	if err != nil || !res2.Available {
		t.Fatalf("second check failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected call count to remain 1 due to cache, got %d", calls)
	}
}

func TestCheckETag304(t *testing.T) {
	mux := http.NewServeMux()
	etagVal := `W/"abc-123"`
	calls := 0
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("If-None-Match") == etagVal {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etagVal)
		w.Header().Set("Content-Type", "application/json")
		w.Write(releaseJSON(t, r, "v2.0.0", []assetFixture{{42, platformAsset(), "/download/42"}}))
	})
	s := fixtureService(t, mux, "1.0.0")

	res1, err := s.Check(context.Background())
	if err != nil || !res1.Available {
		t.Fatalf("first check failed: %v", err)
	}

	// Expire cache time artificially to test ETag
	s.mu.Lock()
	s.cachedAt = s.cachedAt.Add(-3 * time.Minute)
	s.mu.Unlock()

	res2, err := s.Check(context.Background())
	if err != nil || !res2.Available {
		t.Fatalf("second check with 304 failed: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestCheckWebRedirectFallback(t *testing.T) {
	mux := http.NewServeMux()
	// API rate limit returns 403
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	// Web redirect fallback returns 302
	mux.HandleFunc("/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/test/repo/releases/tag/v3.0.0")
		w.WriteHeader(http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := NewService("test/repo", "1.0.0")
	s.baseURL = srv.URL
	s.webBaseURL = srv.URL
	s.client = srv.Client()

	res, err := s.Check(context.Background())
	if err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}
	if !res.Available || res.LatestVersion != "3.0.0" {
		t.Fatalf("expected update to 3.0.0, got %+v", res)
	}
	if !res.AssetFound {
		t.Fatalf("expected synthetic asset to be found, got false")
	}
}

func TestHandleApplyUpdateArgs(t *testing.T) {
	if HandleApplyUpdateArgs([]string{"--foo"}) {
		t.Fatal("expected HandleApplyUpdateArgs to return false for normal args")
	}

	dir := t.TempDir()
	staging := filepath.Join(dir, "tamestack-update-1234")
	if err := os.MkdirAll(staging, 0755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}
	src := filepath.Join(staging, "TameStack.exe")
	dst := filepath.Join(dir, "TameStack.exe")
	if err := os.WriteFile(src, []byte("NEW-BIN"), 0755); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(dst, []byte("OLD-BIN"), 0755); err != nil {
		t.Fatalf("write dst: %v", err)
	}

	handled := HandleApplyUpdateArgs([]string{
		"--apply-update",
		"--source", src,
		"--target", dst,
		"--wait-pid", "0",
	})
	if !handled {
		t.Fatal("expected HandleApplyUpdateArgs to handle --apply-update")
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "NEW-BIN" {
		t.Fatalf("expected target to contain NEW-BIN, got %q", got)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("expected staging dir to be cleaned up, got err=%v", err)
	}
}

func TestInstallWebRedirectFallbackWithExpandedAssetsDigest(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("self-update swap is Windows-only")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "TameStack.exe")
	if err := os.WriteFile(target, []byte("OLD-EXE"), 0755); err != nil {
		t.Fatal(err)
	}

	exeBytes := []byte("NEW-WEB-EXE")
	sum := sha256.Sum256(exeBytes)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/test/repo/releases/tag/v3.0.0")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/test/repo/releases/download/v3.0.0/"+platformAsset(), func(w http.ResponseWriter, r *http.Request) {
		w.Write(exeBytes)
	})
	mux.HandleFunc("/test/repo/releases/expanded_assets/v3.0.0", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<clipboard-copy aria-label="Copy to clipboard digest for %s" type="button" value="%s"></clipboard-copy>`, platformAsset(), digest)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := NewService("test/repo", "1.0.0")
	s.baseURL = srv.URL
	s.webBaseURL = srv.URL
	s.client = srv.Client()
	s.target = target

	if err := s.Install(context.Background(), nil); err != nil {
		t.Fatalf("Install failed with web fallback: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "NEW-WEB-EXE" {
		t.Fatalf("binary not updated: %q %v", got, err)
	}
}

func TestInstallWebRedirectFallbackDigestMismatchAborts(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("self-update swap is Windows-only")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "TameStack.exe")
	if err := os.WriteFile(target, []byte("OLD-EXE"), 0755); err != nil {
		t.Fatal(err)
	}

	exeBytes := []byte("NEW-WEB-EXE")
	wrongDigest := "sha256:" + hex.EncodeToString(make([]byte, 32))

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/test/repo/releases/tag/v3.0.0")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/test/repo/releases/download/v3.0.0/"+platformAsset(), func(w http.ResponseWriter, r *http.Request) {
		w.Write(exeBytes)
	})
	mux.HandleFunc("/test/repo/releases/expanded_assets/v3.0.0", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<clipboard-copy aria-label="Copy to clipboard digest for %s" type="button" value="%s"></clipboard-copy>`, platformAsset(), wrongDigest)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := NewService("test/repo", "1.0.0")
	s.baseURL = srv.URL
	s.webBaseURL = srv.URL
	s.client = srv.Client()
	s.target = target

	if err := s.Install(context.Background(), nil); err == nil {
		t.Fatal("expected digest mismatch error")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "OLD-EXE" {
		t.Fatalf("target modified despite mismatch: %q", got)
	}
}

func TestInstallAssetAPIRateLimitedFallsBackToWeb(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("self-update swap is Windows-only")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "TameStack.exe")
	if err := os.WriteFile(target, []byte("OLD-EXE"), 0755); err != nil {
		t.Fatal(err)
	}

	exeBytes := []byte("NEW-API-FALLBACK-EXE")
	sum := sha256.Sum256(exeBytes)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Write(releaseJSON(t, r, "v9.9.9", []assetFixture{{42, platformAsset(), "/download/42"}}))
	})
	mux.HandleFunc("/repos/test/repo/releases/assets/42", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/download/42", func(w http.ResponseWriter, r *http.Request) {
		w.Write(exeBytes)
	})
	mux.HandleFunc("/test/repo/releases/expanded_assets/v9.9.9", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<clipboard-copy aria-label="Copy to clipboard digest for %s" type="button" value="%s"></clipboard-copy>`, platformAsset(), digest)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := NewService("test/repo", "1.0.0")
	s.baseURL = srv.URL
	s.webBaseURL = srv.URL
	s.client = srv.Client()
	s.target = target

	if err := s.Install(context.Background(), nil); err != nil {
		t.Fatalf("Install failed when asset API rate-limited: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "NEW-API-FALLBACK-EXE" {
		t.Fatalf("binary not updated: %q %v", got, err)
	}
}
