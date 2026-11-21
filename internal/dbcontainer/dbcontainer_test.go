package dbcontainer

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func randomPlain(t *testing.T, size int) []byte {
	t.Helper()
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	return buf
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCreateUnlockRoundTrip(t *testing.T) {
	dir := t.TempDir()
	payloads := map[string]int{
		"empty.db": 0,
		"small.db": 4096,
		"multi.db": chunkPlainSize*2 + 12345,
		"exact.db": chunkPlainSize,
	}
	for name, size := range payloads {
		src := writeFile(t, dir, name, randomPlain(t, size))
		container := filepath.Join(dir, name+".enc")
		if err := Create(container, src, "master-pass"); err != nil {
			t.Fatalf("%s create: %v", name, err)
		}
		if !Exists(container) {
			t.Fatalf("%s: container missing", name)
		}
		stored, err := os.ReadFile(container)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(stored, []byte("SQLite format 3")) {
			t.Fatalf("%s: container leaks plaintext", name)
		}
		dest := filepath.Join(dir, name+".out")
		if err := Unlock(container, dest, "master-pass"); err != nil {
			t.Fatalf("%s unlock: %v", name, err)
		}
		got, err := os.ReadFile(dest)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := os.ReadFile(src)
		if !bytes.Equal(got, want) {
			t.Fatalf("%s: unlocked bytes differ", name)
		}
	}
}

func TestUnlockRejectsWrongPassword(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "db", randomPlain(t, 1000))
	container := filepath.Join(dir, "db.enc")
	if err := Create(container, src, "right-pass"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	if err := Unlock(container, dest, "wrong-pass"); err != ErrInvalidPasswordOrCorrupt {
		t.Fatalf("unlock err = %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("partial output left after failed unlock")
	}
	if err := VerifyPassword(container, "wrong-pass"); err != ErrInvalidPasswordOrCorrupt {
		t.Fatalf("verify err = %v", err)
	}
	if err := VerifyPassword(container, "right-pass"); err != nil {
		t.Fatalf("verify right pass: %v", err)
	}
}

func TestTamperAndTruncateDetected(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "db", randomPlain(t, chunkPlainSize+10))
	container := filepath.Join(dir, "db.enc")
	if err := Create(container, src, "pass"); err != nil {
		t.Fatal(err)
	}

	tampered := filepath.Join(dir, "db.tampered.enc")
	data, _ := os.ReadFile(container)
	data[len(data)-40] ^= 0xFF
	if err := os.WriteFile(tampered, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Unlock(tampered, filepath.Join(dir, "out1"), "pass"); err != ErrInvalidPasswordOrCorrupt {
		t.Fatalf("tampered unlock err = %v", err)
	}

	truncated := filepath.Join(dir, "db.trunc.enc")
	if err := os.WriteFile(truncated, data[:len(data)/2], 0600); err != nil {
		t.Fatal(err)
	}
	if err := Unlock(truncated, filepath.Join(dir, "out2"), "pass"); err != ErrInvalidContainer {
		t.Fatalf("truncated unlock err = %v", err)
	}

	junk := filepath.Join(dir, "junk.enc")
	writeFile(t, dir, "junk.raw", randomPlain(t, 500))
	_ = junk
	if err := Unlock(filepath.Join(dir, "junk.raw"), filepath.Join(dir, "out3"), "pass"); err != ErrInvalidContainer {
		t.Fatalf("junk unlock err = %v", err)
	}
}

func TestSealReplacesContainerWithUpdatedData(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "db", []byte("version-one"))
	container := filepath.Join(dir, "db.enc")
	if err := Create(container, src, "pass"); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(container)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "db", []byte("version-two-longer-payload"))
	if err := Seal(container, src, "pass"); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(container)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("seal did not rewrite the container")
	}

	out := filepath.Join(dir, "out")
	if err := Unlock(container, out, "pass"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	if string(got) != "version-two-longer-payload" {
		t.Fatalf("unlocked = %q", got)
	}
	// salt preserved across seals: wrong password still rejected.
	if err := VerifyPassword(container, "nope"); err != ErrInvalidPasswordOrCorrupt {
		t.Fatalf("verify after seal err = %v", err)
	}
}

func TestDistinctContainersUseDistinctSalts(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "db", randomPlain(t, 100))
	a := filepath.Join(dir, "a.enc")
	b := filepath.Join(dir, "b.enc")
	if err := Create(a, src, "same-pass"); err != nil {
		t.Fatal(err)
	}
	if err := Create(b, src, "same-pass"); err != nil {
		t.Fatal(err)
	}
	da, _ := os.ReadFile(a)
	db, _ := os.ReadFile(b)
	if bytes.Equal(da, db) {
		t.Fatal("two containers built from the same plaintext are byte-identical")
	}
}
