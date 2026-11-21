package ssl

import "testing"

func TestCertbotPluginPackage(t *testing.T) {
	cases := map[string]string{
		"nginx":  "python3-certbot-nginx",
		"apache": "python3-certbot-apache",
		"none":   "python3-certbot-nginx",
		"":       "python3-certbot-nginx",
	}
	for ws, want := range cases {
		if got := certbotPluginPackage(ws); got != want {
			t.Errorf("certbotPluginPackage(%q) = %q, want %q", ws, got, want)
		}
	}
}

func TestParseCertbotDetect(t *testing.T) {
	out := "INSTALLED:yes\nPATH:/usr/bin/certbot\nVERSION:certbot 2.9.0\nRENEW:yes\nWEBSERVER:nginx\n"
	st := parseCertbotDetect(out)
	if !st.Installed || st.Path != "/usr/bin/certbot" || st.Version != "certbot 2.9.0" || !st.RenewTimerActive || st.WebServer != "nginx" {
		t.Errorf("parseCertbotDetect installed case = %+v", st)
	}

	st = parseCertbotDetect("INSTALLED:no\nRENEW:no\nWEBSERVER:none\n")
	if st.Installed || st.RenewTimerActive || st.WebServer != "none" {
		t.Errorf("parseCertbotDetect missing case = %+v", st)
	}

	st = parseCertbotDetect("INSTALLED:yes\nWEBSERVER:bogus\n")
	if st.WebServer != "none" {
		t.Errorf("parseCertbotDetect should ignore unknown webserver, got %q", st.WebServer)
	}
}

func TestParseCLIIni(t *testing.T) {
	iniContent := `# Comment line
email = test@example.com
key-type = rsa
rsa-key-size = 2048
agree-tos = true
deploy-hook = systemctl reload nginx
`
	cfg := parseCLIIni(iniContent)
	if cfg.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", cfg.Email)
	}
	if cfg.KeyType != "rsa" {
		t.Errorf("expected key-type rsa, got %s", cfg.KeyType)
	}
	if cfg.RSAKeySize != 2048 {
		t.Errorf("expected rsa-key-size 2048, got %d", cfg.RSAKeySize)
	}
	if !cfg.AgreeTOS {
		t.Errorf("expected agree-tos true")
	}
	if cfg.DeployHook != "systemctl reload nginx" {
		t.Errorf("expected deploy-hook 'systemctl reload nginx', got %s", cfg.DeployHook)
	}
}

func TestParseCertbotCertificates(t *testing.T) {
	rawOutput := `Saving debug log to /var/log/letsencrypt/letsencrypt.log

- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
Found the following certs:
  Certificate Name: example.com
    Serial Number: 4b2f1a9c
    Key Type: ECDSA
    Domains: example.com www.example.com
    Expiry Date: 2026-05-18 12:34:56+00:00 (VALID: 88 days)
    Certificate Path: /etc/letsencrypt/live/example.com/fullchain.pem
    Private Key Path: /etc/letsencrypt/live/example.com/privkey.pem
  Certificate Name: expired.site
    Serial Number: 99999
    Key Type: RSA
    Domains: expired.site
    Expiry Date: 2026-01-01 00:00:00+00:00 (INVALID: EXPIRED)
    Certificate Path: /etc/letsencrypt/live/expired.site/fullchain.pem
    Private Key Path: /etc/letsencrypt/live/expired.site/privkey.pem
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -`

	certs := parseCertbotCertificates(rawOutput)
	if len(certs) != 2 {
		t.Fatalf("expected 2 certificates, got %d", len(certs))
	}

	c1 := certs[0]
	if c1.Name != "example.com" || c1.KeyType != "ECDSA" || len(c1.Domains) != 2 || c1.DaysLeft != 88 || c1.IsExpired {
		t.Errorf("unexpected cert 1: %+v", c1)
	}

	c2 := certs[1]
	if c2.Name != "expired.site" || c2.KeyType != "RSA" || !c2.IsExpired || c2.DaysLeft != 0 {
		t.Errorf("unexpected cert 2: %+v", c2)
	}
}