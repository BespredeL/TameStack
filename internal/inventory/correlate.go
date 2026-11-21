package inventory

import (
	"fmt"
	"strings"

	"tamestack/internal/db"
	"tamestack/internal/modules/php"
)

// CorrelateLinks checks verified links between sites, PHP-FPM pools, sockets, and SSL certificates.
// Unmatched relationships are flagged as unproven (ADR-S4).
func CorrelateLinks(sites []*db.SiteRecord, pools []php.FpmPoolConfig, certs []*db.SSLCertificateRecord) []VerifiedLink {
	poolSockets := make(map[string]string) // socket path -> pool name
	for _, p := range pools {
		if p.Listen != "" {
			cleanSock := strings.TrimPrefix(p.Listen, "unix:")
			poolSockets[cleanSock] = p.PoolName
		}
	}

	siteCerts := make(map[string]*db.SSLCertificateRecord) // domain -> cert
	for _, c := range certs {
		if c.PrimaryDomain != "" {
			siteCerts[strings.ToLower(c.PrimaryDomain)] = c
		}
		for _, san := range c.SanDomains {
			siteCerts[strings.ToLower(san)] = c
		}
	}

	var links []VerifiedLink
	for _, s := range sites {
		link := VerifiedLink{
			SiteDomain: s.Domain,
			VhostPath:  s.VhostPath,
			Status:     "proven",
		}

		if s.SiteType == "php" || s.SiteType == "laravel" || s.SiteType == "wordpress" {
			expectedSock := fmt.Sprintf("/run/php/php%s-fpm.sock", s.PHPVersion)
			link.FPMSocket = expectedSock
			if poolName, ok := poolSockets[expectedSock]; ok {
				link.PoolName = poolName
			} else if len(pools) > 0 {
				link.Status = "unproven"
				link.Note = fmt.Sprintf("PHP socket %s не найден среди активных пулов FPM", expectedSock)
			}
		}

		if s.SSLEnabled {
			if cert, ok := siteCerts[strings.ToLower(s.Domain)]; ok {
				link.CertPath = cert.CertPath
			} else {
				link.Status = "unproven"
				if link.Note != "" {
					link.Note += "; "
				}
				link.Note += "SSL включен в настройках сайта, но сертификат в базе не сопоставлен"
			}
		}

		links = append(links, link)
	}

	return links
}
