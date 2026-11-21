package secretredact

import "testing"

func TestScrubSecretsFormat(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{
			"Connecting to DB with password=MySecret123 host=localhost",
			"Connecting to DB with password=[REDACTED_SECRET] host=localhost",
		},
		{
			"-----BEGIN RSA PRIVATE KEY-----\nabc\n-----END RSA PRIVATE KEY-----",
			"[REDACTED_PRIVATE_KEY]",
		},
	}
	for _, c := range cases {
		if got := ScrubSecrets(c.in); got != c.want {
			t.Errorf("ScrubSecrets(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestScrubLogTextFormat(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{`password="SuperSecret123"`, `password="[REDACTED_SECRET]"`},
		{"sudo_secret= hunter2", `sudo_secret="[REDACTED_SECRET]"`},
		{"Authorization: Bearer abc123XYZ", "Authorization: Bearer [REDACTED_TOKEN]"},
		{
			"-----BEGIN RSA PRIVATE KEY-----\nabc\n-----END RSA PRIVATE KEY-----",
			"[REDACTED_PRIVATE_KEY]",
		},
	}
	for _, c := range cases {
		if got := ScrubLogText(c.in); got != c.want {
			t.Errorf("ScrubLogText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPrivateKeyRedactionShared(t *testing.T) {
	pem := "-----BEGIN OPENSSH PRIVATE KEY-----\nsecret\n-----END OPENSSH PRIVATE KEY-----"
	if a, b := ScrubSecrets(pem), ScrubLogText(pem); a != b || a != "[REDACTED_PRIVATE_KEY]" {
		t.Errorf("private-key redaction diverged: ScrubSecrets=%q ScrubLogText=%q", a, b)
	}
}
