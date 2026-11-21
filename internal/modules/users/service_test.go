package users

import (
	"testing"
)

func TestValidateUsername(t *testing.T) {
	valid := []string{
		"myuser",
		"site_user",
		"client-1",
		"u123",
	}
	for _, u := range valid {
		if err := ValidateUsername(u); err != nil {
			t.Errorf("expected valid username %s, got error: %v", u, err)
		}
	}

	invalid := []string{
		"",
		"root",
		"daemon",
		"www-data",
		"sudo",
		"1user", // starts with digit
		"-user",
		"user with space",
		"user;rm",
		"very_long_username_that_exceeds_32_characters_limit_123456",
	}
	for _, u := range invalid {
		if err := ValidateUsername(u); err == nil {
			t.Errorf("expected error for invalid username %s, got nil", u)
		}
	}
}

func TestValidateSSHKey(t *testing.T) {
	valid := []string{
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC... user@host",
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... test",
		"ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABB... key",
	}
	for _, k := range valid {
		if err := ValidateSSHKey(k); err != nil {
			t.Errorf("expected valid key, got error: %v", err)
		}
	}

	invalid := []string{
		"",
		"invalid-key-data",
		"-----BEGIN RSA PRIVATE KEY-----",
		"curl http://evil.com/key",
	}
	for _, k := range invalid {
		if err := ValidateSSHKey(k); err == nil {
			t.Errorf("expected error for invalid key, got nil")
		}
	}
}
