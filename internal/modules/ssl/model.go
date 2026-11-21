package ssl

// IssueSSLInput contains parameters for requesting a Let's Encrypt SSL certificate.
type IssueSSLInput struct {
	ServerID      string   `json:"server_id"`
	SiteID        string   `json:"site_id"`
	PrimaryDomain string   `json:"primary_domain"`
	SanDomains    []string `json:"san_domains"`
	Email         string   `json:"email"`
	ChallengeType string   `json:"challenge_type"` // "http-01", "dns-01"
	AutoRenew     bool     `json:"auto_renew"`
}

// SSLMetrics provides counts of certificates and expiration status.
type SSLMetrics struct {
	TotalCertificates int `json:"total_certificates"`
	ValidCount        int `json:"valid_count"`
	ExpiringSoonCount int `json:"expiring_soon_count"` // < 30 days
	ExpiredCount      int `json:"expired_count"`
}
