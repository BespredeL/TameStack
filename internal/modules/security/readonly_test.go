package security

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// A zero-value Service is enough for these tests: every read-only entry point validates its
// arguments before it touches the SSH pool.
func TestValidateConfigReadPath(t *testing.T) {
	allowed := []string{
		"/etc/nginx/nginx.conf",
		"/etc/apache2/ports.conf",
		"/etc/php/8.2/fpm/pool.d/www.conf",
		"/var/log/nginx/error.log",
		"/run/php/php8.2-fpm.sock",
	}
	for _, p := range allowed {
		clean, err := validateConfigReadPath(p)
		if err != nil {
			t.Errorf("validateConfigReadPath(%q) rejected: %v", p, err)
			continue
		}
		if clean != p {
			t.Errorf("validateConfigReadPath(%q) rewrote the path to %q", p, clean)
		}
	}

	rejected := []string{
		"/etc/../root/.ssh/id_rsa",
		"/etc/ssl/private/privkey.pem",
		"/etc/shadow",
		"/home/user/my_secret.txt",
		"/proc/self/environ",
		"nginx.conf",
		"~/.ssh/config",
		"/etc/nginx/$(id).conf",
		"/etc/nginx/`id`.conf",
		"/etc/nginx/nginx.conf; rm -rf /",
		"/etc/php/../../../root/.ssh/id_rsa",
	}
	for _, p := range rejected {
		if clean, err := validateConfigReadPath(p); err == nil {
			t.Errorf("validateConfigReadPath(%q) accepted, resolved to %q", p, clean)
		}
	}
}

func TestConfigReadPrefixesExcludeProc(t *testing.T) {
	for _, prefix := range configReadPrefixes {
		if strings.HasPrefix(prefix, "/proc") || strings.HasPrefix(prefix, "/root") || strings.HasPrefix(prefix, "/home") {
			t.Errorf("config read prefix too broad: %s", prefix)
		}
	}
}

func TestReadLogTailRejectsHostileLogType(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()

	for _, logType := range []string{"$(id)", "error2", "access; rm -rf /", "", "error\naccess"} {
		if _, err := svc.ReadLogTail(ctx, "srv-1", LogTargetNginx, logType, 50); err == nil {
			t.Errorf("ReadLogTail accepted log type %q", logType)
		}
	}
}

func TestReadServiceHelpersRejectHostileUnitNames(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()

	for _, name := range []string{"nginx; id", "nginx$(id)", "nginx|tee", "-nginx", "nginx && reboot", ""} {
		if _, err := svc.ReadServiceStatus(ctx, "srv-1", name); err == nil {
			t.Errorf("ReadServiceStatus accepted service name %q", name)
		}
		if _, err := svc.ReadServiceJournal(ctx, "srv-1", name, 50); err == nil {
			t.Errorf("ReadServiceJournal accepted service name %q", name)
		}
	}
}

func TestReadPHPEnvironmentRejectsHostileVersion(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()

	for _, version := range []string{"8.1; id", "8.1$(id)", "8.1 `id`", "8.1\n8.2"} {
		if _, err := svc.ReadPHPEnvironment(ctx, "srv-1", version); err == nil {
			t.Errorf("ReadPHPEnvironment accepted version %q", version)
		}
	}
}

func TestCheckWebserverSyntaxRejectsUnknownTarget(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()

	if _, err := svc.CheckWebserverSyntax(ctx, "srv-1", "nginx; id"); err == nil {
		t.Error("CheckWebserverSyntax accepted arbitrary webserver type")
	}
}

func TestLogTailScriptsAreCodeOwned(t *testing.T) {
	lines := clampLines(5000, maxInspectionLines)
	if lines != maxInspectionLines {
		t.Fatalf("clampLines(5000, %d) = %d", maxInspectionLines, lines)
	}

	script := firstExistingTailScript(logFilesByTarget[LogTargetNginx]["error"], lines, "Логи не обнаружены.")
	if !strings.Contains(script, "/var/log/nginx/error.log") {
		t.Fatalf("tail script does not use the code-owned nginx log path:\n%s", script)
	}
	if strings.Contains(script, "$(") || strings.Contains(script, "`") {
		t.Errorf("tail script contains shell expansion:\n%s", script)
	}
	if !strings.Contains(script, "LINES=200") {
		t.Errorf("clamped line count not applied:\n%s", script)
	}

	for _, logType := range []string{"error", "access"} {
		auto := autoLogTailScript(logType, lines)
		if strings.Contains(auto, "$(") || strings.Contains(auto, "`") {
			t.Errorf("auto tail script for %s contains shell expansion:\n%s", logType, auto)
		}
		for _, target := range []LogTarget{LogTargetNginx, LogTargetApache, LogTargetPHPFPM} {
			if _, ok := logFilesByTarget[target][logType]; !ok {
				t.Errorf("log family %s has no files for %s", target, logType)
			}
		}
	}
}

func TestClampLines(t *testing.T) {
	cases := map[[2]int]int{{0, 200}: 50, {-5, 200}: 50, {1000, 200}: 200, {17, 200}: 17}
	for in, want := range cases {
		if got := clampLines(in[0], in[1]); got != want {
			t.Errorf("clampLines(%d, %d) = %d, want %d", in[0], in[1], got, want)
		}
	}
}

// TestNoArbitraryCommandAPI guards the boundary the AI tools depend on: read-only inspection is
// reachable only through typed methods, never by handing a command string to the security service.
func TestNoArbitraryCommandAPI(t *testing.T) {
	typ := reflect.TypeOf(&Service{})
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		if strings.Contains(name, "Exec") || strings.Contains(name, "RunCommand") || strings.Contains(name, "Shell") {
			t.Errorf("security.Service exposes %s, which accepts a caller-supplied command", name)
		}
	}
}
