package php

import (
	"strings"
	"testing"
)

func TestParseIniSettings(t *testing.T) {
	raw := `
; Sample php.ini
upload_max_filesize = 64M
post_max_size = 64M
memory_limit = 512M
max_execution_time = 120
max_input_time = 90
max_input_vars = 2000
date.timezone = Europe/Berlin
error_reporting = E_ALL
display_errors = On
log_errors = 1
opcache.enable = 1
opcache.memory_consumption = 256
opcache.max_accelerated_files = 20000
opcache.revalidate_freq = 0
short_open_tag = Off
expose_php = Off
`

	settings := ParseIniSettings(raw)
	if settings.UploadMaxFilesize != "64M" {
		t.Errorf("got upload_max_filesize=%s, want 64M", settings.UploadMaxFilesize)
	}
	if settings.PostMaxSize != "64M" {
		t.Errorf("got post_max_size=%s, want 64M", settings.PostMaxSize)
	}
	if settings.MemoryLimit != "512M" {
		t.Errorf("got memory_limit=%s, want 512M", settings.MemoryLimit)
	}
	if settings.MaxExecutionTime != 120 {
		t.Errorf("got max_execution_time=%d, want 120", settings.MaxExecutionTime)
	}
	if settings.DateTimezone != "Europe/Berlin" {
		t.Errorf("got date.timezone=%s, want Europe/Berlin", settings.DateTimezone)
	}
	if !settings.DisplayErrors {
		t.Errorf("expected display_errors=true")
	}
	if settings.OpcacheMemory != 256 {
		t.Errorf("got opcache.memory_consumption=%d, want 256", settings.OpcacheMemory)
	}
	if settings.ShortOpenTag {
		t.Errorf("expected short_open_tag=false")
	}
}

func TestApplySettingsToIniContent(t *testing.T) {
	existing := `; Original config
memory_limit = 128M
upload_max_filesize = 16M
`
	s := PHPSettings{
		MemoryLimit:       "1024M",
		UploadMaxFilesize: "128M",
		PostMaxSize:       "128M",
		MaxExecutionTime:  300,
		DateTimezone:      "UTC",
	}

	updated := ApplySettingsToIniContent(existing, s)
	if !strings.Contains(updated, "memory_limit = 1024M") {
		t.Errorf("expected updated memory_limit, got:\n%s", updated)
	}
	if !strings.Contains(updated, "upload_max_filesize = 128M") {
		t.Errorf("expected updated upload_max_filesize, got:\n%s", updated)
	}
	if !strings.Contains(updated, "post_max_size = 128M") {
		t.Errorf("expected appended post_max_size, got:\n%s", updated)
	}
}
