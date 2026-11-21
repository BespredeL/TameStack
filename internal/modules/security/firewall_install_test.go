package security

import "testing"

func TestNormalizeFirewallType(t *testing.T) {
	cases := map[string]string{
		"ufw":         "ufw",
		"UFW":         "ufw",
		"firewalld":   "firewalld",
		"firewall-cmd": "firewalld",
		" Firewalld ": "firewalld",
	}
	for in, want := range cases {
		got, err := normalizeFirewallType(in)
		if err != nil || got != want {
			t.Errorf("normalizeFirewallType(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "iptables", "ufw; rm -rf /", "nftables"} {
		if _, err := normalizeFirewallType(bad); err == nil {
			t.Errorf("normalizeFirewallType(%q) expected error", bad)
		}
	}
}

func TestFirewallInstallSpec(t *testing.T) {
	pkgs, svc := firewallInstallSpec("ufw")
	if len(pkgs) != 1 || pkgs[0] != "ufw" || svc != "ufw" {
		t.Errorf("ufw spec = %v, %q", pkgs, svc)
	}
	pkgs, svc = firewallInstallSpec("firewalld")
	if len(pkgs) != 1 || pkgs[0] != "firewalld" || svc != "firewalld" {
		t.Errorf("firewalld spec = %v, %q", pkgs, svc)
	}
}