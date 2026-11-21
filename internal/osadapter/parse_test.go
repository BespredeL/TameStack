package osadapter

import (
	"reflect"
	"testing"
)

const hardwareOutput = `===CPU_CORES===
4
===CPU_MODEL===
Intel(R) Xeon(R) CPU E5-2680 v4 @ 2.40GHz
===RAM_TOTAL===
8354623488
===DISK_INFO===
107374182400 53687091200
===UPTIME===
123456
===SSH_VER===
OpenSSH_8.9p1 Ubuntu-3ubuntu0.10, OpenSSL 3.0.2 15 Mar 2022
`

func TestParseHardwareOutput(t *testing.T) {
	hw := parseHardwareOutput(hardwareOutput)
	if hw.CPUCores != 4 {
		t.Errorf("cores = %d, want 4", hw.CPUCores)
	}
	if hw.CPUModel != "Intel(R) Xeon(R) CPU E5-2680 v4 @ 2.40GHz" {
		t.Errorf("model = %q", hw.CPUModel)
	}
	if hw.RAMTotalBytes != 8354623488 {
		t.Errorf("ram = %d", hw.RAMTotalBytes)
	}
	if hw.DiskTotalBytes != 107374182400 || hw.DiskUsedBytes != 53687091200 {
		t.Errorf("disk = %d/%d", hw.DiskTotalBytes, hw.DiskUsedBytes)
	}
	if hw.UptimeSeconds != 123456 {
		t.Errorf("uptime = %d", hw.UptimeSeconds)
	}
	if hw.SSHVersion != "OpenSSH_8.9p1 Ubuntu-3ubuntu0.10, OpenSSL 3.0.2 15 Mar 2022" {
		t.Errorf("ssh = %q", hw.SSHVersion)
	}
}

func TestParseHardwareOutputMalformed(t *testing.T) {
	for _, out := range []string{"", "garbage without sections", "===CPU_CORES===", "===DISK_INFO===\nonlyonefield\n"} {
		hw := parseHardwareOutput(out)
		if hw == nil {
			t.Fatal("nil result")
		}
		if hw.DiskTotalBytes != 0 || hw.DiskUsedBytes != 0 {
			t.Errorf("disk fields should stay zero for %q", out)
		}
	}
}

const softwareOutput = `===WEBSERVERS===
nginx
apache2
===PHP===
8.1
8.3
===DB===
mysql
postgresql
===DOCKER===
Docker version 24.0.7, build afdd53b
`

func TestParseSoftwareOutput(t *testing.T) {
	sw := parseSoftwareOutput(softwareOutput, true)
	if !reflect.DeepEqual(sw.WebServers, []string{"nginx", "apache2"}) {
		t.Errorf("webservers = %v", sw.WebServers)
	}
	if !reflect.DeepEqual(sw.PHPVersions, []string{"8.1", "8.3"}) {
		t.Errorf("php = %v", sw.PHPVersions)
	}
	if !reflect.DeepEqual(sw.DBVersions, []string{"mysql", "postgresql"}) {
		t.Errorf("db = %v", sw.DBVersions)
	}
	if sw.DockerVersion != "Docker version 24.0.7, build afdd53b" {
		t.Errorf("docker = %q", sw.DockerVersion)
	}
}

func TestParseSoftwareOutputEmptySections(t *testing.T) {
	sw := parseSoftwareOutput("===WEBSERVERS===\n===PHP===\n===DB===\n===DOCKER===\n", true)
	if sw.WebServers == nil || sw.PHPVersions == nil || sw.DBVersions == nil {
		t.Error("slices must be non-nil empty")
	}
	if len(sw.WebServers) != 0 || len(sw.PHPVersions) != 0 || len(sw.DBVersions) != 0 || sw.DockerVersion != "" {
		t.Error("expected empty result")
	}
}

func TestParseSoftwareOutputPHPGlob(t *testing.T) {
	out := "===PHP===\n/etc/php/*\n8.2\n"
	debian := parseSoftwareOutput(out, true)
	if !reflect.DeepEqual(debian.PHPVersions, []string{"8.2"}) {
		t.Errorf("debian php = %v, want glob filtered", debian.PHPVersions)
	}
	rhel := parseSoftwareOutput(out, false)
	if len(rhel.PHPVersions) != 2 {
		t.Errorf("rhel php = %v, want glob kept", rhel.PHPVersions)
	}
}

const ubuntuRelease = `NAME="Ubuntu"
VERSION_ID="22.04"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04.4 LTS"
5.15.0-105-generic
x86_64`

func TestParseOSReleaseUbuntu(t *testing.T) {
	info := parseOSRelease(ubuntuRelease)
	if info.Family != FamilyDebian || info.Distro != "ubuntu" || info.Version != "22.04" {
		t.Errorf("info = %+v", info)
	}
	if info.PrettyName != "Ubuntu 22.04.4 LTS" {
		t.Errorf("pretty = %q", info.PrettyName)
	}
	if info.Kernel != "5.15.0-105-generic" || info.Arch != "x86_64" {
		t.Errorf("kernel/arch = %q/%q", info.Kernel, info.Arch)
	}
}

func TestParseOSReleaseTrailingNewline(t *testing.T) {
	info := parseOSRelease(ubuntuRelease + "\n")
	if info.Arch != "x86_64" || info.Kernel != "5.15.0-105-generic" {
		t.Errorf("kernel/arch = %q/%q", info.Kernel, info.Arch)
	}
}

func TestParseOSReleaseRhelFamily(t *testing.T) {
	cases := []struct {
		name string
		out  string
	}{
		{"almalinux", "ID=\"almalinux\"\nID_LIKE=\"rhel centos fedora\"\nVERSION_ID=\"9.3\"\n5.14.0-362.el9.x86_64\nx86_64"},
		{"rocky", "ID=rocky\nVERSION_ID=\"9.3\"\n5.14.0-362.el9.x86_64\nx86_64"},
		{"centos-idlike", "ID=centos\nID_LIKE=\"rhel fedora\"\nVERSION_ID=\"8\"\n4.18.0-513.el8.x86_64\nx86_64"},
		{"fedora", "ID=fedora\nVERSION_ID=39\n6.5.6-300.fc39.x86_64\nx86_64"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := parseOSRelease(tc.out)
			if info.Family != FamilyRhel {
				t.Errorf("family = %q, want rhel", info.Family)
			}
		})
	}
}

func TestParseOSReleaseFallbackAndPrettyName(t *testing.T) {
	info := parseOSRelease("ID=arch\nNAME=\"Arch Linux\"\nVERSION_ID=\"rolling\"\n6.6.1-arch1-1\nx86_64")
	if info.Family != FamilyDebian {
		t.Errorf("unknown distro must fall back to debian, got %q", info.Family)
	}
	if info.PrettyName != "Arch Linux rolling" {
		t.Errorf("pretty fallback = %q", info.PrettyName)
	}
}
