package site

import (
	"bytes"
	"fmt"
	"text/template"
)

// RenderNginxConfig generates an optimized Nginx server block using the embedded template.
func RenderNginxConfig(in SiteInput) string {
	engine := &NginxEngine{}
	out, err := engine.BuildVirtualHost(in)
	if err != nil {
		return fmt.Sprintf("# Error generating Nginx config: %v", err)
	}
	return out
}

// RenderApacheConfig generates a production-ready Apache 2.4 VirtualHost configuration using the embedded template.
func RenderApacheConfig(in SiteInput) string {
	engine := &ApacheEngine{}
	out, err := engine.BuildVirtualHost(in)
	if err != nil {
		return fmt.Sprintf("# Error generating Apache config: %v", err)
	}
	return out
}

// RenderDefaultIndex creates a professional placeholder page if DocumentRoot is empty.
func RenderDefaultIndex(in SiteInput) (string, string) {
	if in.SiteType == "static" {
		tmplContent, err := siteFS.ReadFile("templates/default_index.html.tmpl")
		if err == nil {
			if tmpl, err := template.New("default_index_html").Parse(string(tmplContent)); err == nil {
				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, in); err == nil {
					return "index.html", buf.String()
				}
			}
		}
		// Fallback
		return "index.html", fmt.Sprintf("<!DOCTYPE html><html><body><h1>%s</h1><p>Website is ready.</p></body></html>", in.Domain)
	}

	tmplContent, err := siteFS.ReadFile("templates/default_index.php.tmpl")
	if err == nil {
		if tmpl, err := template.New("default_index_php").Parse(string(tmplContent)); err == nil {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, in); err == nil {
				return "index.php", buf.String()
			}
		}
	}
	// Fallback
	return "index.php", fmt.Sprintf("<?php echo 'Website %s is ready.';", in.Domain)
}
