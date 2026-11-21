package capabilities

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOSReleaseFixtures(t *testing.T) {
	cases := []struct {
		file            string
		expectedID      string
		expectedVersion string
		isLTS           bool
		allowInstall    bool
	}{
		{"ubuntu_18.04.os-release", "ubuntu", "18.04", true, true},
		{"ubuntu_20.04.os-release", "ubuntu", "20.04", true, true},
		{"ubuntu_22.04.os-release", "ubuntu", "22.04", true, true},
		{"ubuntu_24.04.os-release", "ubuntu", "24.04", true, true},
		{"ubuntu_26.04.os-release", "ubuntu", "26.04", true, true},
		{"ubuntu_24.10.os-release", "ubuntu", "24.10", false, false},
		{"debian_bookworm.os-release", "debian", "12", false, true},
	}

	for _, tc := range cases {
		content, err := os.ReadFile(filepath.Join("testdata", tc.file))
		if err != nil {
			t.Fatalf("Failed to read testdata %s: %v", tc.file, err)
		}
		rel := ParseOSRelease(string(content))
		if rel.ID != tc.expectedID {
			t.Errorf("[%s] expected ID=%s, got=%s", tc.file, tc.expectedID, rel.ID)
		}
		if rel.VersionID != tc.expectedVersion {
			t.Errorf("[%s] expected VersionID=%s, got=%s", tc.file, tc.expectedVersion, rel.VersionID)
		}
		if rel.ID == "ubuntu" {
			if IsUbuntuLTS(rel.VersionID) != tc.isLTS {
				t.Errorf("[%s] expected IsUbuntuLTS=%v, got=%v", tc.file, tc.isLTS, IsUbuntuLTS(rel.VersionID))
			}
		}
		ok, _ := AllowsPackageInstall(string(content))
		if ok != tc.allowInstall {
			t.Errorf("[%s] expected AllowsPackageInstall=%v, got=%v", tc.file, tc.allowInstall, ok)
		}
	}
}

func TestAllowsPHPInstallRules(t *testing.T) {
	bionic, _ := os.ReadFile(filepath.Join("testdata", "ubuntu_18.04.os-release"))
	focal, _ := os.ReadFile(filepath.Join("testdata", "ubuntu_20.04.os-release"))
	noble, _ := os.ReadFile(filepath.Join("testdata", "ubuntu_24.04.os-release"))
	resolute, _ := os.ReadFile(filepath.Join("testdata", "ubuntu_26.04.os-release"))
	nonLts, _ := os.ReadFile(filepath.Join("testdata", "ubuntu_24.10.os-release"))

	// 18.04 only allows 7.2.
	if ok, _ := AllowsPHPInstall(string(bionic), "8.3"); ok {
		t.Fatal("18.04 should forbid PHP 8.3")
	}
	if ok, _ := AllowsPHPInstall(string(bionic), "7.2"); !ok {
		t.Fatal("18.04 should allow PHP 7.2")
	}

	// 20.04 allows 7.4 through 8.3 via PPA, but forbids 8.4+.
	if ok, _ := AllowsPHPInstall(string(focal), "8.3"); !ok {
		t.Fatal("20.04 should allow PHP 8.3")
	}
	if ok, _ := AllowsPHPInstall(string(focal), "7.4"); !ok {
		t.Fatal("20.04 should allow PHP 7.4")
	}
	if ok, _ := AllowsPHPInstall(string(focal), "8.4"); ok {
		t.Fatal("20.04 should forbid PHP 8.4")
	}

	// 24.04 allows 8.3.
	if ok, _ := AllowsPHPInstall(string(noble), "8.3"); !ok {
		t.Fatal("24.04 should allow PHP 8.3")
	}

	// 26.04 forbids bare metapackage "php"
	if ok, _ := AllowsPHPInstall(string(resolute), "php"); ok {
		t.Fatal("26.04 should forbid bare php metapackage")
	}
	if ok, _ := AllowsPHPInstall(string(resolute), "8.5"); !ok {
		t.Fatal("26.04 should allow php 8.5")
	}

	// non-LTS blocks install.
	if ok, _ := AllowsPHPInstall(string(nonLts), "8.3"); ok {
		t.Fatal("non-LTS 24.10 should forbid PHP install")
	}
}

func TestMySQLSyntaxCapability(t *testing.T) {
	if AllowsMySQLCreateUserIfNotExists("mysql Ver 14.14 Distrib 5.7.35, for Linux") {
		t.Fatal("MySQL 5.7 should not allow CREATE USER IF NOT EXISTS")
	}
	if !AllowsMySQLCreateUserIfNotExists("mysql Ver 8.0.36-0ubuntu0.22.04.1 for Linux") {
		t.Fatal("MySQL 8.0 should allow CREATE USER IF NOT EXISTS")
	}
	if !AllowsMySQLCreateUserIfNotExists("mariadb from 11.2.2-MariaDB, client 15.2") {
		t.Fatal("MariaDB should allow CREATE USER IF NOT EXISTS")
	}
}

func TestParseAptCachePolicy(t *testing.T) {
	outputFound := `php8.2-fpm:
  Installed: (none)
  Candidate: 8.2.20-1+ubuntu22.04.1+deb.sury.org+1
  Version table:
     8.2.20-1+ubuntu22.04.1+deb.sury.org+1 500
        500 https://ppa.launchpadcontent.net/ondrej/php/ubuntu jammy/main amd64 Packages
`
	res := ParseAptCachePolicy(outputFound)
	if !res.Found || res.Candidate != "8.2.20-1+ubuntu22.04.1+deb.sury.org+1" {
		t.Fatalf("expected candidate found, got %+v", res)
	}

	outputNotFound := `N: Unable to locate package nonexistent-pkg
`
	res2 := ParseAptCachePolicy(outputNotFound)
	if res2.Found {
		t.Fatalf("expected candidate not found, got %+v", res2)
	}
}

func TestProbeParsers(t *testing.T) {
	nginxOut := `nginx version: nginx/1.18.0 (Ubuntu)
built with OpenSSL 1.1.1f  31 Mar 2020
TLS SNI support enabled
configure arguments: --with-cc-opt='-g -O2' --with-http_ssl_module --with-http_v2_module
`
	nginxRes := ParseNginxV(nginxOut)
	if nginxRes.Version != "1.18.0" || !nginxRes.HasHTTP2 || !nginxRes.HasSSL {
		t.Fatalf("unexpected nginx probe result: %+v", nginxRes)
	}

	if v := ParsePHPV("PHP 8.2.18 (cli) (built: Apr  8 2024 15:48:47) (NTS)"); v != "8.2.18" {
		t.Fatalf("unexpected php version: %s", v)
	}
	if v := ParseCertbotV("certbot 2.6.0"); v != "2.6.0" {
		t.Fatalf("unexpected certbot version: %s", v)
	}
	if v := ParseDockerV("Docker version 24.0.5, build ced0996"); v != "24.0.5" {
		t.Fatalf("unexpected docker version: %s", v)
	}
	if v := ParseSystemctlV("systemd 249 (249.11-0ubuntu3.12)"); v != "249" {
		t.Fatalf("unexpected systemd version: %s", v)
	}
}
