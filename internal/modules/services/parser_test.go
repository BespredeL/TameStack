package services

import (
	"testing"
)

func TestIsProtectedUnit(t *testing.T) {
	// Should protect SSH and Network from stop/disable.
	if err := IsProtectedUnit("ssh.service", "stop"); err == nil {
		t.Errorf("expected error when stopping ssh.service, got nil")
	}
	if err := IsProtectedUnit("sshd", "disable"); err == nil {
		t.Errorf("expected error when disabling sshd, got nil")
	}
	if err := IsProtectedUnit("systemd-networkd.service", "stop"); err == nil {
		t.Errorf("expected error when stopping systemd-networkd, got nil")
	}

	// Should allow restart of SSH.
	if err := IsProtectedUnit("ssh.service", "restart"); err != nil {
		t.Errorf("expected nil when restarting ssh.service, got %v", err)
	}

	// Should allow stop/disable of regular services.
	if err := IsProtectedUnit("nginx.service", "stop"); err != nil {
		t.Errorf("expected nil when stopping nginx.service, got %v", err)
	}
	if err := IsProtectedUnit("php8.2-fpm.service", "disable"); err != nil {
		t.Errorf("expected nil when disabling php-fpm, got %v", err)
	}
}

func TestValidateProcessKill(t *testing.T) {
	// PID 1 protection.
	if err := ValidateProcessKill(1, "SIGTERM"); err == nil {
		t.Errorf("expected error for PID 1, got nil")
	}
	if err := ValidateProcessKill(0, "SIGKILL"); err == nil {
		t.Errorf("expected error for PID 0, got nil")
	}
	if err := ValidateProcessKill(-5, "SIGKILL"); err == nil {
		t.Errorf("expected error for negative PID, got nil")
	}

	// Valid process kill.
	if err := ValidateProcessKill(1234, "SIGTERM"); err != nil {
		t.Errorf("expected valid for PID 1234, got %v", err)
	}
	if err := ValidateProcessKill(5678, "SIGKILL"); err != nil {
		t.Errorf("expected valid for PID 5678, got %v", err)
	}

	// Invalid signal
	if err := ValidateProcessKill(1234, "INVALID_SIGNAL"); err == nil {
		t.Errorf("expected error for invalid signal, got nil")
	}
}

func TestParseSystemctlUnits(t *testing.T) {
	sample := `
cron.service                         loaded active running Regular background program processing daemon
nginx.service                        loaded active running A high performance web server and a reverse proxy server
php8.2-fpm.service                   loaded active running The PHP 8.2 FastCGI Process Manager
ssh.service                          loaded active running OpenBSD Secure Shell server
certbot.timer                        loaded active waiting Run certbot twice daily
systemd-journald.socket              loaded active running Journal Socket
basic.target                         loaded active active  Basic System
`
	units, err := ParseSystemctlUnits(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(units) != 7 {
		t.Fatalf("expected 7 units, got %d", len(units))
	}

	if units[1].Unit != "nginx.service" || units[1].UnitType != "service" || units[1].Sub != "running" {
		t.Errorf("unexpected unit 1: %+v", units[1])
	}
	if units[4].Unit != "certbot.timer" || units[4].UnitType != "timer" {
		t.Errorf("unexpected unit 4: %+v", units[4])
	}
	if units[5].Unit != "systemd-journald.socket" || units[5].UnitType != "socket" {
		t.Errorf("unexpected unit 5: %+v", units[5])
	}
}

func TestParsePsOutput(t *testing.T) {
	sample := `USER                     PID  %CPU %MEM    VSZ   RSS STAT START     TIME COMMAND
root                       1   0.0  0.2 168340 12968 Ss   Nov10   0:04 /sbin/init
root                     567   0.5  0.8  45620 32100 S    10:00   0:02 /usr/sbin/nginx -g daemon on;
www-data                 568   1.2  1.4  46100 42000 S    10:00   0:05 nginx: worker process
`
	procs, err := ParsePsOutput(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(procs) != 3 {
		t.Fatalf("expected 3 procs, got %d", len(procs))
	}

	if procs[0].PID != 1 || procs[0].User != "root" || procs[0].Command != "/sbin/init" {
		t.Errorf("unexpected proc 0: %+v", procs[0])
	}
	if procs[1].PID != 567 || procs[1].CPU != 0.5 {
		t.Errorf("unexpected proc 1: %+v", procs[1])
	}
}

func TestParseSsListening(t *testing.T) {
	sample := `Netid State Recv-Q Send-Q Local Address:Port Peer Address:PortProcess
tcp   LISTEN 0      511          0.0.0.0:80        0.0.0.0:*    users:(("nginx",pid=567,fd=6))
tcp   LISTEN 0      128          0.0.0.0:22        0.0.0.0:*    users:(("sshd",pid=890,fd=3))
`
	ports, err := ParseSsListening(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}
	if ports[0].LocalPort != 80 || ports[0].Process != "nginx" || ports[0].PID != 567 {
		t.Errorf("unexpected port 0: %+v", ports[0])
	}
	if ports[1].LocalPort != 22 || ports[1].Process != "sshd" || ports[1].PID != 890 {
		t.Errorf("unexpected port 1: %+v", ports[1])
	}
}
