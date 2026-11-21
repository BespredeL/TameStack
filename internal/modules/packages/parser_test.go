package packages

import (
	"testing"
)

func TestValidatePackageNames(t *testing.T) {
	valid := [][]string{
		{"nginx"},
		{"php8.2-fpm", "curl", "git"},
		{"libssl3", "libapache2-mod-php", "mysql-server-8.0"},
	}
	for _, pkgs := range valid {
		if err := ValidatePackageNames(pkgs); err != nil {
			t.Errorf("expected valid packages %v, got error: %v", pkgs, err)
		}
	}

	invalid := [][]string{
		{""},
		{"nginx; rm -rf /"},
		{"nginx && apt-get"},
		{"`whoami`"},
		{"pkg|grep"},
		{"$HOME"},
		{"pkg$(id)"},
		{"-pkg"}, // leading dash not allowed by regex
		{"pkg with space"},
	}
	for _, pkgs := range invalid {
		if err := ValidatePackageNames(pkgs); err == nil {
			t.Errorf("expected error for invalid packages %v, got nil", pkgs)
		}
	}
}

func TestParseAptSimulation(t *testing.T) {
	sampleOutput := `
NOTE: This is only a simulation!
      apt-get needs root privileges for real execution.
      Keep also in mind that locking is deactivated,
      so don't depend on the relevance to the real current situation!
The following additional packages will be installed:
  libnginx-mod-http-geoip2 nginx-common
Suggested packages:
  fcgiwrap nginx-doc
The following NEW packages will be installed:
  libnginx-mod-http-geoip2 nginx nginx-common
The following packages will be upgraded:
  curl libcurl4
Inst libcurl4 [7.81.0-1ubuntu1.15] (7.81.0-1ubuntu1.16 Ubuntu:22.04/jammy-updates [amd64])
Inst curl [7.81.0-1ubuntu1.15] (7.81.0-1ubuntu1.16 Ubuntu:22.04/jammy-updates [amd64])
Inst nginx-common (1.18.0-6ubuntu14.4 Ubuntu:22.04/jammy-updates [all])
Inst libnginx-mod-http-geoip2 (1.18.0-6ubuntu14.4 Ubuntu:22.04/jammy-updates [amd64])
Inst nginx (1.18.0-6ubuntu14.4 Ubuntu:22.04/jammy-updates [amd64])
Conf libcurl4 (7.81.0-1ubuntu1.16 Ubuntu:22.04/jammy-updates [amd64])
Conf curl (7.81.0-1ubuntu1.16 Ubuntu:22.04/jammy-updates [amd64])
Remv old-package [1.0.0]
2 upgraded, 3 newly installed, 1 to remove and 0 not upgraded.
`
	result, err := ParseAptSimulation(sampleOutput, "apt-get -s install nginx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.UpgradedCount != 2 {
		t.Errorf("expected 2 upgraded, got %d", result.UpgradedCount)
	}
	if result.InstalledCount != 3 {
		t.Errorf("expected 3 newly installed, got %d", result.InstalledCount)
	}
	if result.RemovedCount != 1 {
		t.Errorf("expected 1 to remove, got %d", result.RemovedCount)
	}

	if len(result.Changes) != 6 {
		t.Fatalf("expected 6 package changes, got %d", len(result.Changes))
	}

	// Verify upgrade
	if result.Changes[0].Name != "libcurl4" || result.Changes[0].Action != "upgrade" ||
		result.Changes[0].OldVersion != "7.81.0-1ubuntu1.15" ||
		result.Changes[0].NewVersion != "7.81.0-1ubuntu1.16" ||
		result.Changes[0].Arch != "amd64" {
		t.Errorf("unexpected change 0: %+v", result.Changes[0])
	}
	// Verify install
	if result.Changes[2].Name != "nginx-common" || result.Changes[2].Action != "install" ||
		result.Changes[2].NewVersion != "1.18.0-6ubuntu14.4" ||
		result.Changes[2].Arch != "all" {
		t.Errorf("unexpected change 2: %+v", result.Changes[2])
	}
	// Verify remove
	if result.Changes[5].Name != "old-package" || result.Changes[5].Action != "remove" ||
		result.Changes[5].OldVersion != "1.0.0" {
		t.Errorf("unexpected change 5: %+v", result.Changes[5])
	}
}

func TestParseDpkgList(t *testing.T) {
	sampleOutput := "nginx\t1.18.0-6ubuntu14.4\tamd64\tinstall ok installed\tsmall, powerful, scalable web/proxy server\n" +
		"curl\t7.81.0-1ubuntu1.15\tamd64\tinstall ok installed\tcommand line tool for transferring data with URL syntax\n" +
		"removed-pkg\t1.0.0\tamd64\tdeinstall ok config-files\tPackage removed\n"

	items, err := ParseDpkgList(sampleOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 installed packages, got %d", len(items))
	}
	if items[0].Name != "nginx" || items[0].Version != "1.18.0-6ubuntu14.4" {
		t.Errorf("unexpected item 0: %+v", items[0])
	}
}
