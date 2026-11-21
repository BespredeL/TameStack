package php

import (
	"strings"
	"testing"
)

func TestRenderRichPoolConfig(t *testing.T) {
	in := CreatePoolInput{
		PoolName:          "site_alpha",
		Version:           "8.3",
		User:              "appuser",
		Group:             "appgroup",
		ListenSocket:      "/run/php/php8.3-fpm-site_alpha.sock",
		ListenOwner:       "appuser",
		ListenGroup:       "www-data",
		ListenMode:        "0660",
		PmMode:            "dynamic",
		MaxChildren:       20,
		StartServers:      4,
		MinSpareServers:   2,
		MaxSpareServers:   6,
		PmMaxRequests:     500,
		Slowlog:           true,
		SlowlogTimeout:    "3s",
		MemoryLimit:       "512M",
		MaxExecutionTime:  90,
		UploadMaxFilesize: "64M",
		PostMaxSize:       "64M",
		PhpValues: map[string]string{
			"error_log": "/var/log/php_errors.log",
		},
		PhpAdminValues: map[string]string{
			"open_basedir": "/var/www/site_alpha:/tmp",
		},
	}

	content, err := RenderPoolConfig(in)
	if err != nil {
		t.Fatalf("RenderPoolConfig failed: %v", err)
	}

	expectedDirectives := []string{
		"[site_alpha]",
		"user = appuser",
		"group = appgroup",
		"listen = /run/php/php8.3-fpm-site_alpha.sock",
		"listen.owner = appuser",
		"listen.group = www-data",
		"listen.mode = 0660",
		"pm = dynamic",
		"pm.max_children = 20",
		"pm.start_servers = 4",
		"pm.min_spare_servers = 2",
		"pm.max_spare_servers = 6",
		"pm.max_requests = 500",
		"slowlog = /var/log/php8.3-fpm-site_alpha.slow.log",
		"request_slowlog_timeout = 3s",
		"php_admin_value[memory_limit] = 512M",
		"php_admin_value[max_execution_time] = 90",
		"php_admin_value[upload_max_filesize] = 64M",
		"php_admin_value[post_max_size] = 64M",
		"php_value[error_log] = /var/log/php_errors.log",
		"php_admin_value[open_basedir] = /var/www/site_alpha:/tmp",
	}

	for _, exp := range expectedDirectives {
		if !strings.Contains(content, exp) {
			t.Errorf("expected directive %q in generated pool config:\n%s", exp, content)
		}
	}
}

func TestValidatePoolNameCases(t *testing.T) {
	valid := []string{"mysite", "site1", "client-a", "shop_prod", "wp_blog"}
	for _, name := range valid {
		if err := ValidatePoolName(name); err != nil {
			t.Errorf("expected valid pool name %q, got err: %v", name, err)
		}
	}

	invalid := []string{"www", "", "SiteCapital", "bad!char", "site space", "123", "-starts-with-dash"}
	for _, name := range invalid {
		if err := ValidatePoolName(name); err == nil {
			t.Errorf("expected rejection for pool name %q, got nil", name)
		}
	}
}
