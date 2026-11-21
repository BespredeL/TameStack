package ssl

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/osadapter"
	"tamestack/internal/ssh"
	"tamestack/internal/tasks"

	"github.com/google/uuid"
)

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// Service manages SSL certificates and Let's Encrypt automation.
type Service struct {
	module.Base
	engine *tasks.Engine
}

// NewService instantiates SSL service and registers task handlers.
func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool, engine *tasks.Engine) *Service {
	s := &Service{
		Base:   module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		engine: engine,
	}

	engine.RegisterHandler("ssl_issue", s.handleIssueSSLTask)
	engine.RegisterHandler("ssl_renew", s.handleRenewSSLTask)
	engine.RegisterHandler("ssl_install_certbot", s.handleInstallCertbotTask)

	return s
}

// ListCertificates returns all SSL certificates for a server.
func (s *Service) ListCertificates(ctx context.Context, serverID string) ([]*db.SSLCertificateRecord, error) {
	return s.Database.ListSSLCertificates(ctx, serverID)
}

// GetMetrics returns statistics on certificate statuses.
func (s *Service) GetMetrics(ctx context.Context, serverID string) (*SSLMetrics, error) {
	certs, err := s.Database.ListSSLCertificates(ctx, serverID)
	if err != nil {
		return nil, err
	}

	metrics := &SSLMetrics{
		TotalCertificates: len(certs),
	}

	for _, c := range certs {
		switch c.Status {
		case "expired":
			metrics.ExpiredCount++
		case "expiring_soon":
			metrics.ExpiringSoonCount++
		default:
			metrics.ValidCount++
		}
	}

	return metrics, nil
}

// IssueCertificate queues a Let's Encrypt certificate issuance task.
func (s *Service) IssueCertificate(ctx context.Context, in IssueSSLInput) (*db.TaskRecord, error) {
	in.PrimaryDomain = strings.TrimSpace(strings.ToLower(in.PrimaryDomain))
	if !domainRegex.MatchString(in.PrimaryDomain) {
		return nil, fmt.Errorf("invalid domain name: %s", in.PrimaryDomain)
	}

	if in.Email == "" {
		in.Email = "admin@" + in.PrimaryDomain
	}
	if in.ChallengeType == "" {
		in.ChallengeType = "http-01"
	}

	payloadBytes, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	title := fmt.Sprintf("Issue Let's Encrypt SSL for %s", in.PrimaryDomain)
	return s.engine.Submit(in.ServerID, "ssl_issue", title, string(payloadBytes))
}

// RenewCertificate queues a renewal task for the certificate.
func (s *Service) RenewCertificate(ctx context.Context, certID string) (*db.TaskRecord, error) {
	cert, err := s.Database.GetSSLCertificate(ctx, certID)
	if err != nil {
		return nil, fmt.Errorf("certificate %s not found: %w", certID, err)
	}

	payloadMap := map[string]string{
		"cert_id":        cert.ID,
		"server_id":      cert.ServerID,
		"primary_domain": cert.PrimaryDomain,
	}
	payloadBytes, _ := json.Marshal(payloadMap)

	title := fmt.Sprintf("Renew SSL certificate for %s", cert.PrimaryDomain)
	return s.engine.Submit(cert.ServerID, "ssl_renew", title, string(payloadBytes))
}

// DeleteCertificate removes a certificate from the database.
func (s *Service) DeleteCertificate(ctx context.Context, certID string) error {
	return s.Database.DeleteSSLCertificate(ctx, certID)
}

func parseOpenSSLDate(dStr string) (*time.Time, error) {
	dStr = strings.TrimSpace(dStr)
	formats := []string{
		"Jan _2 15:04:05 2006 MST",
		"Jan  2 15:04:05 2006 MST",
		"Jan 2 15:04:05 2006 MST",
		"Jan _2 15:04:05 2006",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		time.RFC822,
		time.RFC1123,
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, dStr); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("unable to parse date: %s", dStr)
}

// DiscoverCertificates inspects certificates on the remote filesystem (Let's Encrypt live, renewals, and webserver configs)
// and synchronizes them with the SQLite database.
func (s *Service) DiscoverCertificates(ctx context.Context, serverID string) ([]*db.SSLCertificateRecord, error) {
	client, _, err := s.GetSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	script := module.SudoPreamble + `
inspect_cert() {
    local cfile="$1"
    local kfile="$2"
    local dname="$3"

    if [ -z "$cfile" ]; then return; fi
    if ! do_sudo test -f "$cfile" 2>/dev/null; then return; fi

    local cert_text
    cert_text=$(do_sudo openssl x509 -in "$cfile" -noout -subject -issuer -startdate -enddate 2>/dev/null)
    if [ -z "$cert_text" ]; then return; fi

    if [ -z "$dname" ]; then
        dname=$(echo "$cert_text" | grep -i "subject=" | sed -E 's/.*(CN|commonName)\s*=\s*([^/,]+).*/\2/' | tr -d ' ')
    fi
    if [ -z "$dname" ] || [ "$dname" = "snakeoil" ]; then
        return
    fi

    local issuer
    issuer=$(echo "$cert_text" | grep -i "issuer=" | sed -E 's/.*(O|organizationName|CN|commonName)\s*=\s*([^/,]+).*/\2/' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    if [ -z "$issuer" ]; then
        issuer="Let's Encrypt"
    fi

    local not_before
    not_before=$(echo "$cert_text" | grep -i "notBefore=" | sed 's/notBefore=//' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

    local not_after
    not_after=$(echo "$cert_text" | grep -i "notAfter=" | sed 's/notAfter=//' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

    local sans
    sans=$(do_sudo openssl x509 -in "$cfile" -noout -text 2>/dev/null | grep -A1 "Subject Alternative Name:" | tail -n1 | tr -d ' ' | sed 's/DNS://g')

    if [ -z "$kfile" ]; then
        local cdir
        cdir=$(dirname "$cfile")
        if do_sudo test -f "$cdir/privkey.pem"; then
            kfile="$cdir/privkey.pem"
        elif do_sudo test -f "$cdir/privkey1.pem"; then
            kfile="$cdir/privkey1.pem"
        elif do_sudo test -f "${cfile%.*}.key"; then
            kfile="${cfile%.*}.key"
        fi
    fi

    echo "===CERT:$dname==="
    echo "CERT_PATH:$cfile"
    echo "KEY_PATH:$kfile"
    echo "ISSUER:$issuer"
    echo "NOT_BEFORE:$not_before"
    echo "NOT_AFTER:$not_after"
    echo "SANS:$sans"
}

SEEN_CERTS=""

# 1. Certbot / snap certbot / letsencrypt CLI inspection
for CB in certbot /snap/bin/certbot /usr/bin/certbot /usr/bin/letsencrypt /usr/local/bin/certbot /opt/certbot/bin/certbot; do
    if command -v "$CB" >/dev/null 2>&1 || [ -x "$CB" ]; then
        CB_OUT=$(do_sudo "$CB" certificates 2>/dev/null)
        if [ -n "$CB_OUT" ]; then
            while IFS= read -r line; do
                if echo "$line" | grep -qi "Certificate Path:"; then
                    cpath=$(echo "$line" | awk '{print $NF}')
                    if [ -n "$cpath" ]; then
                        case " $SEEN_CERTS " in
                            *" $cpath "*) ;;
                            *)
                                SEEN_CERTS="$SEEN_CERTS $cpath"
                                kpath=$(echo "$cpath" | sed 's/fullchain\.pem/privkey.pem/; s/cert\.pem/privkey.pem/')
                                inspect_cert "$cpath" "$kpath" ""
                                ;;
                        esac
                    fi
                fi
            done <<< "$CB_OUT"
        fi
        break
    fi
done

# 2. Check /etc/letsencrypt/renewal/*.conf
for rconf in $(do_sudo ls /etc/letsencrypt/renewal/*.conf 2>/dev/null); do
    cpath=$(do_sudo grep -E '^\s*(fullchain|cert)\s*=' "$rconf" 2>/dev/null | head -n1 | awk -F'=' '{print $2}' | tr -d ' ')
    kpath=$(do_sudo grep -E '^\s*privkey\s*=' "$rconf" 2>/dev/null | head -n1 | awk -F'=' '{print $2}' | tr -d ' ')
    dname=$(basename "$rconf" .conf)
    if [ -n "$cpath" ]; then
        case " $SEEN_CERTS " in
            *" $cpath "*) ;;
            *)
                SEEN_CERTS="$SEEN_CERTS $cpath"
                inspect_cert "$cpath" "$kpath" "$dname"
                ;;
        esac
    fi
done

# 3. Check /etc/letsencrypt/live/
for d in $(do_sudo ls -d /etc/letsencrypt/live/* 2>/dev/null); do
    domain=$(basename "$d")
    if [ "$domain" = "README" ]; then continue; fi
    for cname in fullchain.pem cert.pem; do
        cpath="$d/$cname"
        if do_sudo test -f "$cpath" 2>/dev/null; then
            case " $SEEN_CERTS " in
                *" $cpath "*) ;;
                *)
                    SEEN_CERTS="$SEEN_CERTS $cpath"
                    inspect_cert "$cpath" "$d/privkey.pem" "$domain"
                    ;;
            esac
            break
        fi
    done
done

# 4. Scan Apache configurations (including *-le-ssl.conf)
for conf in /etc/apache2/sites-enabled/*.conf /etc/apache2/sites-available/*.conf /etc/httpd/conf.d/*.conf /etc/apache2/conf.d/*.conf; do
    if do_sudo test -f "$conf" 2>/dev/null; then
        while IFS= read -r line; do
            cpath=$(echo "$line" | awk '{print $2}' | tr -d '";' | tr -d "'")
            if [ -n "$cpath" ]; then
                case " $SEEN_CERTS " in
                    *" $cpath "*) ;;
                    *)
                        SEEN_CERTS="$SEEN_CERTS $cpath"
                        kpath=$(do_sudo grep -E '^\s*SSLCertificateKeyFile' "$conf" 2>/dev/null | head -n1 | awk '{print $2}' | tr -d '";' | tr -d "'")
                        inspect_cert "$cpath" "$kpath" ""
                        ;;
                esac
            fi
        done < <(do_sudo grep -E '^\s*SSLCertificateFile' "$conf" 2>/dev/null)
    fi
done

# 5. Scan Nginx configurations
for conf in /etc/nginx/sites-enabled/* /etc/nginx/sites-available/* /etc/nginx/conf.d/*.conf; do
    if do_sudo test -f "$conf" 2>/dev/null; then
        while IFS= read -r line; do
            cpath=$(echo "$line" | awk '{print $2}' | tr -d '";' | tr -d "'")
            if [ -n "$cpath" ]; then
                case " $SEEN_CERTS " in
                    *" $cpath "*) ;;
                    *)
                        SEEN_CERTS="$SEEN_CERTS $cpath"
                        kpath=$(do_sudo grep -E '^\s*ssl_certificate_key' "$conf" 2>/dev/null | head -n1 | awk '{print $2}' | tr -d '";' | tr -d "'")
                        inspect_cert "$cpath" "$kpath" ""
                        ;;
                esac
            fi
        done < <(do_sudo grep -E '^\s*ssl_certificate\s+' "$conf" 2>/dev/null)
    fi
done

# 6. Scan /etc/ssl/certs/ for custom domain certs
for c in $(do_sudo find /etc/ssl/certs /etc/ssl/localcerts -maxdepth 2 -type f \( -name "*.crt" -o -name "*.pem" \) 2>/dev/null); do
    b=$(basename "$c")
    if [ "$b" = "ssl-cert-snakeoil.pem" ] || [ "$b" = "ca-certificates.crt" ]; then continue; fi
    case " $SEEN_CERTS " in
        *" $c "*) ;;
        *)
            SEEN_CERTS="$SEEN_CERTS $c"
            inspect_cert "$c" "" ""
            ;;
    esac
done
`
	out, _, _, _ := client.NewCommand(script).WithSudo().Exec(ctx)
	outStr := string(out)

	existingList, _ := s.Database.ListSSLCertificates(ctx, serverID)
	existingMap := make(map[string]*db.SSLCertificateRecord)
	for _, c := range existingList {
		existingMap[c.PrimaryDomain] = c
	}

	sections := strings.Split(outStr, "===CERT:")
	for _, sec := range sections {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}
		parts := strings.SplitN(sec, "===", 2)
		if len(parts) < 2 {
			continue
		}
		domain := strings.TrimSpace(parts[0])
		lines := strings.Split(parts[1], "\n")

		var certPath, keyPath, issuer string
		var validFrom, validTo *time.Time
		var sans []string

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "CERT_PATH:") {
				certPath = strings.TrimPrefix(line, "CERT_PATH:")
			} else if strings.HasPrefix(line, "KEY_PATH:") {
				keyPath = strings.TrimPrefix(line, "KEY_PATH:")
			} else if strings.HasPrefix(line, "ISSUER:") {
				issuer = strings.TrimPrefix(line, "ISSUER:")
			} else if strings.HasPrefix(line, "issuer=") {
				issuer = strings.TrimPrefix(line, "issuer=")
			} else if strings.HasPrefix(line, "NOT_BEFORE:") {
				dStr := strings.TrimSpace(strings.TrimPrefix(line, "NOT_BEFORE:"))
				if t, err := parseOpenSSLDate(dStr); err == nil {
					validFrom = t
				}
			} else if strings.HasPrefix(line, "notBefore=") {
				dStr := strings.TrimSpace(strings.TrimPrefix(line, "notBefore="))
				if t, err := parseOpenSSLDate(dStr); err == nil {
					validFrom = t
				}
			} else if strings.HasPrefix(line, "NOT_AFTER:") {
				dStr := strings.TrimSpace(strings.TrimPrefix(line, "NOT_AFTER:"))
				if t, err := parseOpenSSLDate(dStr); err == nil {
					validTo = t
				}
			} else if strings.HasPrefix(line, "notAfter=") {
				dStr := strings.TrimSpace(strings.TrimPrefix(line, "notAfter="))
				if t, err := parseOpenSSLDate(dStr); err == nil {
					validTo = t
				}
			} else if strings.HasPrefix(line, "SANS:") {
				rawSans := strings.TrimPrefix(line, "SANS:")
				for _, s := range strings.Split(rawSans, ",") {
					s = strings.TrimSpace(s)
					if s != "" && s != domain {
						sans = append(sans, s)
					}
				}
			}
		}

		if domain == "" && len(sans) > 0 {
			domain = sans[0]
		}

		if domain == "" || certPath == "" {
			continue
		}

		daysRemaining := 90
		status := "valid"
		if validTo != nil {
			daysRemaining = int(time.Until(*validTo).Hours() / 24)
			if daysRemaining < 0 {
				status = "expired"
			} else if daysRemaining < 30 {
				status = "expiring_soon"
			}
		}
		if issuer == "" {
			issuer = "Let's Encrypt"
		}

		if existing, found := existingMap[domain]; found {
			existing.CertPath = certPath
			existing.KeyPath = keyPath
			existing.Issuer = issuer
			existing.ValidFrom = validFrom
			existing.ValidTo = validTo
			existing.DaysRemaining = daysRemaining
			existing.Status = status
			existing.SanDomains = sans
			existing.UpdatedAt = time.Now()
			_ = s.Database.SaveSSLCertificate(ctx, existing)
		} else {
			now := time.Now()
			newCert := &db.SSLCertificateRecord{
				ID:            uuid.New().String(),
				ServerID:      serverID,
				PrimaryDomain: domain,
				SanDomains:    sans,
				CertPath:      certPath,
				KeyPath:       keyPath,
				Issuer:        issuer,
				ValidFrom:     validFrom,
				ValidTo:       validTo,
				DaysRemaining: daysRemaining,
				AutoRenew:     true,
				Status:        status,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			_ = s.Database.SaveSSLCertificate(ctx, newCert)
		}
	}

	// Synchronize SSL status for matching sites in database.
	if sites, err := s.Database.ListSites(ctx, serverID); err == nil {
		allCerts, _ := s.Database.ListSSLCertificates(ctx, serverID)
		for _, site := range sites {
			for _, cert := range allCerts {
				isMatch := strings.EqualFold(site.Domain, cert.PrimaryDomain)
				if !isMatch {
					for _, san := range cert.SanDomains {
						if strings.EqualFold(site.Domain, san) {
							isMatch = true
							break
						}
					}
				}
				if isMatch {
					site.SSLEnabled = true
					if cert.Issuer != "" {
						site.SSLProvider = cert.Issuer
					} else {
						site.SSLProvider = "letsencrypt"
					}
					_ = s.Database.SaveSite(ctx, site)
					break
				}
			}
		}
	}

	return s.Database.ListSSLCertificates(ctx, serverID)
}

// handleIssueSSLTask automates Certbot installation, domain challenge, and Nginx binding.
func (s *Service) handleIssueSSLTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var in IssueSSLInput
	if err := json.Unmarshal([]byte(task.Payload), &in); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}

	logger.SetStep("Connecting to server and checking Certbot installation", 15)
	client, record, err := s.GetSSHClient(ctx, in.ServerID)
	if err != nil {
		return "", err
	}

	// Prefer the site's recorded web server; fall back to what is actually running.
	isApacheActive := false
	resolvedFromSite := false
	if in.SiteID != "" {
		if site, err := s.Database.GetSite(ctx, in.SiteID); err == nil && site != nil && site.WebServer != "" {
			ws := strings.ToLower(site.WebServer)
			resolvedFromSite = true
			switch {
			case strings.Contains(ws, "nginx") && strings.Contains(ws, "apache"):
				isApacheActive = false // hybrid: nginx is the public front for certbot
			case strings.Contains(ws, "apache"):
				isApacheActive = true
			default:
				isApacheActive = false
			}
		}
	}
	if !resolvedFromSite {
		outWeb, _, _, _ := client.Exec(ctx, "systemctl is-active apache2 2>/dev/null || systemctl is-active httpd 2>/dev/null || true")
		isApacheActive = strings.TrimSpace(string(outWeb)) == "active"
		if !isApacheActive {
			outPgrep, _, _, _ := client.Exec(ctx, "pgrep -f 'apache2|httpd' 2>/dev/null || true")
			isApacheActive = strings.TrimSpace(string(outPgrep)) != ""
		}
	}

	// Verify certbot presence across standard system and snap locations.
	outCheck, _, codeCheck, _ := client.NewCommand("export PATH=$PATH:/snap/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin && (which certbot || which letsencrypt || test -x /snap/bin/certbot || test -x /usr/bin/certbot || test -x /usr/bin/letsencrypt)").Exec(ctx)
	if codeCheck != 0 || len(strings.TrimSpace(string(outCheck))) == 0 {
		logger.Info("Certbot is not installed. Installing certbot and plugins...")
		adapter, _, err := osadapter.DetectAndCreateAdapter(ctx, client)
		if err != nil {
			return "", fmt.Errorf("failed to detect OS for certbot install: %w", err)
		}
		plugin := "python3-certbot-nginx"
		if isApacheActive {
			plugin = "python3-certbot-apache"
		}
		if adapter.GetOSInfo().Family != osadapter.FamilyRhel {
			if _, err := adapter.UpdatePackageIndex(ctx, client); err != nil {
				return "", fmt.Errorf("failed to install certbot: %w", err)
			}
		}
		outInstall, err := adapter.InstallPackages(ctx, client, []string{"certbot", plugin})
		if err != nil {
			return "", fmt.Errorf("failed to install certbot: %s %v", outInstall, err)
		}
		logger.Info("Certbot successfully installed.")
	}

	logger.SetStep(fmt.Sprintf("Requesting Let's Encrypt certificate for %s", in.PrimaryDomain), 35)

	domainArgs := []string{"-d", in.PrimaryDomain}
	for _, san := range in.SanDomains {
		san = strings.TrimSpace(strings.ToLower(san))
		if san != "" && san != in.PrimaryDomain && domainRegex.MatchString(san) {
			domainArgs = append(domainArgs, "-d", san)
		}
	}

	var certbotCmd string
	if isApacheActive {
		certbotCmd = fmt.Sprintf(
			"export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin && certbot certonly --apache --non-interactive --agree-tos --email %s %s 2>&1",
			ssh.ShellEscape(in.Email),
			strings.Join(domainArgs, " "),
		)
	} else {
		certbotCmd = fmt.Sprintf(
			"export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin && certbot certonly --nginx --non-interactive --agree-tos --email %s %s 2>&1",
			ssh.ShellEscape(in.Email),
			strings.Join(domainArgs, " "),
		)
	}

	logger.Info(fmt.Sprintf("Executing certbot for %s...", in.PrimaryDomain))
	outCertbot, _, codeCertbot, errCertbot := client.NewCommand(certbotCmd).WithSudo().Exec(ctx)
	logger.Info(string(outCertbot))

	if errCertbot != nil || codeCertbot != 0 {
		return "", fmt.Errorf("Certbot verification failed (code %d):\n%s\nPlease ensure port 80 is open and domain points to %s",
			codeCertbot, string(outCertbot), record.Host)
	}

	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", in.PrimaryDomain)
	keyPath := fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", in.PrimaryDomain)

	logger.SetStep("Verifying generated certificate files and expiration", 65)
	// Read certificate enddate via openssl.
	outDate, _, codeDate, _ := client.NewCommand(fmt.Sprintf("openssl x509 -enddate -noout -in %s", certPath)).WithSudo().Exec(ctx)
	var validTo time.Time
	daysRemaining := 90
	if codeDate == 0 && strings.Contains(string(outDate), "notAfter=") {
		// Output format: notAfter=May 12 12:00:00 2026 GMT.
		dateStr := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(outDate)), "notAfter="))
		if t, err := parseOpenSSLDate(dateStr); err == nil && t != nil {
			validTo = *t
			daysRemaining = int(time.Until(validTo).Hours() / 24)
		}
	}

	now := time.Now()
	certRecord := &db.SSLCertificateRecord{
		ID:            uuid.New().String(),
		ServerID:      in.ServerID,
		SiteID:        in.SiteID,
		PrimaryDomain: in.PrimaryDomain,
		SanDomains:    in.SanDomains,
		CertPath:      certPath,
		KeyPath:       keyPath,
		Issuer:        "Let's Encrypt",
		ValidFrom:     &now,
		ValidTo:       &validTo,
		DaysRemaining: daysRemaining,
		AutoRenew:     in.AutoRenew,
		Status:        "valid",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Step: Bind to WebServer VirtualHost.
	if isApacheActive {
		logger.SetStep("Binding SSL certificate to Apache VirtualHost", 80)
		apacheConf := fmt.Sprintf("/etc/apache2/sites-available/%s.conf", in.PrimaryDomain)
		if outCheck, _, _, _ := client.Exec(ctx, fmt.Sprintf("[ -f %s ] && echo 'exists'", apacheConf)); strings.TrimSpace(string(outCheck)) != "exists" {
			apacheConf = fmt.Sprintf("/etc/httpd/conf.d/%s.conf", in.PrimaryDomain)
		}
		vhostData, err := client.ReadFile(ctx, apacheConf)
		if err == nil {
			vhostStr := string(vhostData)
			if !strings.Contains(vhostStr, "SSLCertificateFile") {
				sslBlock := fmt.Sprintf(`
<VirtualHost *:443>
    ServerName %s
    SSLEngine on
    SSLCertificateFile %s
    SSLCertificateKeyFile %s
</VirtualHost>
`, in.PrimaryDomain, certPath, keyPath)
				_ = client.WriteFile(ctx, apacheConf+".pre-ssl", []byte(vhostStr), 0644)
				_ = client.WriteFile(ctx, apacheConf, []byte(vhostStr+"\n"+sslBlock), 0644)
				_, _, _, _ = client.NewCommand("apache2ctl configtest 2>/dev/null || apachectl configtest 2>/dev/null || true").WithSudo().Exec(ctx)
				_, _, _, _ = client.NewCommand("systemctl reload apache2 2>/dev/null || systemctl reload httpd 2>/dev/null || true").WithSudo().Exec(ctx)
				logger.Info("Apache successfully updated with SSL directives.")
			}
		}
	} else {
		logger.SetStep("Binding SSL certificate to Nginx VirtualHost", 80)
		vhostPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", in.PrimaryDomain)
		vhostData, err := client.ReadFile(ctx, vhostPath)
		if err == nil {
			vhostStr := string(vhostData)
			if !strings.Contains(vhostStr, "ssl_certificate") {
				// Inject SSL configuration block into Nginx server block.
				sslDirectives := fmt.Sprintf(`
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    ssl_certificate %s;
    ssl_certificate_key %s;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
`, certPath, keyPath)

				// Replace `listen 80;` with redirect block and HTTPS server block.
				redirectBlock := fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;
    return 301 https://$host$request_uri;
}
`, in.PrimaryDomain)

				// Insert SSL directives inside the existing server block after server_name.
				idx := strings.Index(vhostStr, "server_name")
				if idx != -1 {
					lineEnd := strings.Index(vhostStr[idx:], ";")
					if lineEnd != -1 {
						insertPos := idx + lineEnd + 1
						newVhost := redirectBlock + "\n" + vhostStr[:insertPos] + sslDirectives + vhostStr[insertPos:]
						// Remove the old listen 80 from the main block.
						newVhost = strings.Replace(newVhost, "listen 80;\n", "", 1)
						newVhost = strings.Replace(newVhost, "listen [::]:80;\n", "", 1)

						// Backup and write.
						_ = client.WriteFile(ctx, vhostPath+".pre-ssl", []byte(vhostStr), 0644)
						_ = client.WriteFile(ctx, vhostPath, []byte(newVhost), 0644)

						// Validate syntax
						testOut, _, codeTest, _ := client.NewCommand("nginx -t 2>&1").WithSudo().Exec(ctx)
						if codeTest == 0 && strings.Contains(string(testOut), "syntax is ok") {
							_, _, _, _ = client.NewCommand("systemctl reload nginx").WithSudo().Exec(ctx)
							logger.Info("Nginx successfully updated with HTTPS redirect and SSL certificate.")
							if in.SiteID != "" {
								_ = s.Database.UpdateSiteStatus(ctx, in.SiteID, "active")
							}
						} else {
							// Rollback
							_ = client.WriteFile(ctx, vhostPath, []byte(vhostStr), 0644)
							logger.Warn(fmt.Sprintf("Nginx SSL test warning, retained HTTP: %s", string(testOut)))
						}
					}
				}
			}
		}
	}

	logger.SetStep("Saving certificate record in database", 95)
	if err := s.Database.SaveSSLCertificate(ctx, certRecord); err != nil {
		logger.Warn(fmt.Sprintf("Failed to save certificate record: %v", err))
	}

	// Synchronize matching site's SSL status in database.
	if in.SiteID != "" {
		if site, err := s.Database.GetSite(ctx, in.SiteID); err == nil && site != nil {
			site.SSLEnabled = true
			site.SSLProvider = "letsencrypt"
			_ = s.Database.SaveSite(ctx, site)
		}
	} else if sites, err := s.Database.ListSites(ctx, in.ServerID); err == nil {
		for _, site := range sites {
			if strings.EqualFold(site.Domain, in.PrimaryDomain) {
				site.SSLEnabled = true
				site.SSLProvider = "letsencrypt"
				_ = s.Database.SaveSite(ctx, site)
				break
			}
		}
	}

	logger.SetStep("SSL certificate issued successfully", 100)
	return fmt.Sprintf("SSL certificate successfully issued for %s (Expires in %d days)", in.PrimaryDomain, daysRemaining), nil
}

// handleRenewSSLTask executes Certbot renewal for the certificate.
func (s *Service) handleRenewSSLTask(ctx context.Context, task *db.TaskRecord, logger tasks.TaskLogger) (string, error) {
	var payload struct {
		CertID        string `json:"cert_id"`
		ServerID      string `json:"server_id"`
		PrimaryDomain string `json:"primary_domain"`
	}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", fmt.Errorf("invalid payload: %w", err)
	}

	logger.SetStep("Connecting to server", 20)
	client, _, err := s.GetSSHClient(ctx, payload.ServerID)
	if err != nil {
		return "", err
	}

	logger.SetStep(fmt.Sprintf("Executing renewal for %s", payload.PrimaryDomain), 50)
	renewScript := fmt.Sprintf(module.SudoPreamble+`
CB_BIN="certbot"
for b in certbot /snap/bin/certbot /usr/bin/certbot /usr/bin/letsencrypt /usr/local/bin/certbot /opt/certbot/bin/certbot; do
    if command -v "$b" >/dev/null 2>&1 || [ -x "$b" ]; then
        CB_BIN="$b"
        break
    fi
done

DOM=%s
CERT_NAME="$DOM"
if do_sudo test -f "/etc/letsencrypt/renewal/${DOM}.conf" 2>/dev/null; then
    CERT_NAME="$DOM"
else
    FOUND_NAME=$(do_sudo ls /etc/letsencrypt/renewal/*"${DOM}"*.conf 2>/dev/null | head -n1)
    if [ -n "$FOUND_NAME" ]; then
        CERT_NAME=$(basename "$FOUND_NAME" .conf)
    fi
fi

OUT=$(do_sudo "$CB_BIN" renew --cert-name "$CERT_NAME" --non-interactive 2>&1)
EXIT_CODE=$?

if [ $EXIT_CODE -ne 0 ]; then
    OUT=$(do_sudo "$CB_BIN" renew --non-interactive 2>&1)
    EXIT_CODE=$?
fi

echo "$OUT"
exit $EXIT_CODE
`, ssh.ShellEscape(payload.PrimaryDomain))

	out, _, code, err := client.NewCommand(renewScript).WithSudo().Exec(ctx)
	logger.Info(string(out))

	if err != nil || code != 0 {
		return "", fmt.Errorf("certbot renewal failed: %s", string(out))
	}

	logger.SetStep("Reloading web servers", 80)
	_, _, _, _ = client.NewCommand("systemctl reload nginx 2>/dev/null || true; systemctl reload apache2 2>/dev/null || systemctl reload httpd 2>/dev/null || true").WithSudo().Exec(ctx)

	// Recalculate days remaining.
	checkCertCmd := fmt.Sprintf(module.SudoPreamble+`for c in /etc/letsencrypt/live/%s/fullchain.pem /etc/letsencrypt/live/%s/cert.pem; do
    if do_sudo test -f "$c" 2>/dev/null; then
        do_sudo openssl x509 -enddate -noout -in "$c" 2>/dev/null
        break
    fi
done
`, ssh.ShellEscape(payload.PrimaryDomain), ssh.ShellEscape(payload.PrimaryDomain))

	outDate, _, codeDate, _ := client.NewCommand(checkCertCmd).WithSudo().Exec(ctx)
	if codeDate == 0 && strings.Contains(string(outDate), "notAfter=") {
		dateStr := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(outDate)), "notAfter="))
		if t, err := parseOpenSSLDate(dateStr); err == nil && t != nil {
			certRecord, _ := s.Database.GetSSLCertificate(ctx, payload.CertID)
			if certRecord != nil {
				certRecord.ValidTo = t
				certRecord.DaysRemaining = int(time.Until(*t).Hours() / 24)
				certRecord.Status = "valid"
				_ = s.Database.SaveSSLCertificate(ctx, certRecord)
			}
		}
	}

	logger.SetStep("Renewal completed", 100)
	return fmt.Sprintf("Certificate for %s renewed successfully", payload.PrimaryDomain), nil
}
