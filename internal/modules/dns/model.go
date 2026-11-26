package dns

// DNSStatus describes the BIND9 (named) installation and managed zones.
type DNSStatus struct {
	Installed   bool             `json:"installed"`
	Active      bool             `json:"active"`
	ServiceName string           `json:"service_name"`
	Zones       []DNSZoneSummary `json:"zones"`
}

// DNSZoneSummary represents a single DNS zone managed by TameStack.
type DNSZoneSummary struct {
	Domain       string `json:"domain"`
	FilePath     string `json:"file_path"`
	RecordsCount int    `json:"records_count"`
}

// DNSRecord represents a single resource record in a BIND zone file.
type DNSRecord struct {
	Name     string `json:"name"`
	TTL      int    `json:"ttl"`
	Type     string `json:"type"`     // A, AAAA, CNAME, MX, TXT, NS, SRV, CAA
	Priority int    `json:"priority"` // used for MX / SRV
	Value    string `json:"value"`
}

// DNSZoneDetail holds the parsed records and raw zone file content.
type DNSZoneDetail struct {
	Domain     string      `json:"domain"`
	FilePath   string      `json:"file_path"`
	Serial     int64       `json:"serial"`
	Records    []DNSRecord `json:"records"`
	RawContent string      `json:"raw_content"`
}

type createZoneRequest struct {
	ServerID string `json:"server_id"`
	Domain   string `json:"domain"`
	ServerIP string `json:"server_ip"`
}

type saveZoneRequest struct {
	ServerID string      `json:"server_id"`
	Domain   string      `json:"domain"`
	Records  []DNSRecord `json:"records"`
}

type deleteZoneRequest struct {
	ServerID string `json:"server_id"`
	Domain   string `json:"domain"`
}
