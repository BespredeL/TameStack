package ssh

import (
	"os"
	"strings"
	"testing"
)

func TestSudoWriteScriptEscapesHostilePaths(t *testing.T) {
	client := &SSHClient{user: "deploy", sudoPassword: "sup3rs3cr3t"}

	script := client.sudoWriteScript("/etc/nginx/$(id).d", "/etc/nginx/conf.d/`x`.conf", "/tmp/.sh_write_1_2", 0644)

	for _, want := range []string{
		"'/etc/nginx/$(id).d'",
		"'/etc/nginx/conf.d/`x`.conf'",
		"'/tmp/.sh_write_1_2'",
		"chmod 0644",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %s:\n%s", want, script)
		}
	}
	if strings.Contains(script, `"/etc`) {
		t.Errorf("path emitted inside double quotes, remote expansion possible:\n%s", script)
	}
	if strings.Contains(script, "sup3rs3cr3t") {
		t.Errorf("sudo password leaked into script:\n%s", script)
	}
}

func TestSudoReadScriptEscapesHostilePaths(t *testing.T) {
	client := &SSHClient{user: "deploy", sudoPassword: "sup3rs3cr3t"}

	script := client.sudoReadScript("/var/log/$(id).log")

	if !strings.Contains(script, "'/var/log/$(id).log'") {
		t.Errorf("path not single-quoted:\n%s", script)
	}
	if strings.Contains(script, `"/var/log/$(id).log"`) {
		t.Errorf("path double-quoted, command substitution still expands:\n%s", script)
	}
	if strings.Contains(script, "sup3rs3cr3t") {
		t.Errorf("sudo password leaked into script:\n%s", script)
	}
	if !strings.Contains(script, "head -c 10485760") {
		t.Errorf("read size limit missing:\n%s", script)
	}
}

func TestSudoScriptsForRootSkipAskpass(t *testing.T) {
	client := &SSHClient{user: "root"}

	writeScript := client.sudoWriteScript("/etc/nginx", "/etc/nginx/nginx.conf", "/tmp/.sh_write_1_2", os.FileMode(0600))
	readScript := client.sudoReadScript("/etc/hosts")

	if strings.Contains(writeScript, "sudo -A") || strings.Contains(readScript, "sudo -A") {
		t.Errorf("root session should not route through askpass sudo:\n%s\n---\n%s", writeScript, readScript)
	}
	if !strings.Contains(writeScript, "chmod 0600") {
		t.Errorf("file mode missing:\n%s", writeScript)
	}
}
