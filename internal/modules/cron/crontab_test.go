package cron

import (
	"strings"
	"testing"
)

func TestCrontabKeepsPercentDirectives(t *testing.T) {
	body := "0 1 * * * date +%F\n"
	cmd, err := buildCrontabCommand("root", body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd, "date +%F") {
		t.Fatalf("percent directive was rewritten: %s", cmd)
	}
	if strings.Contains(cmd, "printf") {
		t.Fatalf("crontab must not be passed through printf: %s", cmd)
	}
}

func TestSplitCrontabKeepsNonstandardLines(t *testing.T) {
	raw := "VARIABLE=value\nFOO=bar\n0 * * * * echo hi\n"
	jobs, preserved := splitCrontab(raw, "root")
	if len(jobs) != 1 || jobs[0].Command != "echo hi" {
		t.Fatalf("jobs: %#v", jobs)
	}
	body := strings.Join(preserved, "\n")
	if !strings.Contains(body, "VARIABLE=value") || !strings.Contains(body, "FOO=bar") {
		t.Fatalf("preserved lines dropped: %#v", preserved)
	}
}

func TestSplitCrontabShortcuts(t *testing.T) {
	raw := "@daily /usr/bin/backup\n@reboot /usr/local/bin/startup.sh\n"
	jobs, _ := splitCrontab(raw, "root")
	if len(jobs) != 2 {
		t.Fatalf("expected 2 shortcut jobs, got %d", len(jobs))
	}
	if jobs[0].Expression != "@daily" || jobs[0].Command != "/usr/bin/backup" {
		t.Errorf("unexpected job 0: %+v", jobs[0])
	}
	if jobs[1].Expression != "@reboot" || jobs[1].Command != "/usr/local/bin/startup.sh" {
		t.Errorf("unexpected job 1: %+v", jobs[1])
	}
}

func TestParseSystemCronFile(t *testing.T) {
	raw := `
# /etc/crontab: system-wide crontab
SHELL=/bin/sh
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin

# Hourly system job
17 *	* * *	root    cd / && run-parts --report /etc/cron.hourly
25 6	* * *	root	test -x /usr/sbin/anacron || ( cd / && run-parts --report /etc/cron.daily )
@daily			root	/usr/local/bin/daily-backup.sh
#DISABLED: 0 0 * * * root /usr/local/bin/nightly.sh
`
	jobs := parseSystemCronFile(raw, "root", "/etc/crontab")
	if len(jobs) != 4 {
		t.Fatalf("expected 4 jobs from system cron, got %d", len(jobs))
	}
	if jobs[0].Expression != "17 * * * *" || jobs[0].User != "root" || !jobs[0].Enabled {
		t.Errorf("job 0 mismatch: %+v", jobs[0])
	}
	if jobs[2].Expression != "@daily" || jobs[2].Command != "/usr/local/bin/daily-backup.sh" || !jobs[2].Enabled {
		t.Errorf("job 2 mismatch: %+v", jobs[2])
	}
	if jobs[3].Expression != "0 0 * * *" || jobs[3].Enabled {
		t.Errorf("job 3 should be disabled: %+v", jobs[3])
	}
}
