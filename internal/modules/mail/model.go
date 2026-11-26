package mail

// MailStatus describes the installed mail stack and configured domains, mailboxes, and aliases.
type MailStatus struct {
	PostfixInstalled  bool         `json:"postfix_installed"`
	PostfixActive     bool         `json:"postfix_active"`
	DovecotInstalled  bool         `json:"dovecot_installed"`
	DovecotActive     bool         `json:"dovecot_active"`
	OpenDKIMInstalled bool         `json:"opendkim_installed"`
	OpenDKIMActive    bool         `json:"opendkim_active"`
	Hostname          string       `json:"hostname"`
	Domains           []MailDomain `json:"domains"`
	Mailboxes         []MailboxItem `json:"mailboxes"`
	Aliases           []MailAlias  `json:"aliases"`
}

// MailDomain represents a virtual mail domain with generated DKIM/SPF/DMARC records.
type MailDomain struct {
	Domain        string `json:"domain"`
	DKIMSelector  string `json:"dkim_selector"`
	DKIMRecord    string `json:"dkim_record"`
	SPFRecord     string `json:"spf_record"`
	DMARCRecord   string `json:"dmarc_record"`
}

// MailboxItem represents a virtual Dovecot/Postfix mailbox.
type MailboxItem struct {
	Email     string `json:"email"`
	Domain    string `json:"domain"`
	LocalPart string `json:"local_part"`
	Maildir   string `json:"maildir"`
	QuotaMB   int    `json:"quota_mb"`
}

// MailAlias represents a Postfix virtual alias mapping.
type MailAlias struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// InstallMailStackRequest holds parameters for installing Postfix + Dovecot + OpenDKIM.
type InstallMailStackRequest struct {
	ServerID string `json:"server_id"`
	Hostname string `json:"hostname"`
}

// MailDomainRequest holds parameters to add or remove a mail domain.
type MailDomainRequest struct {
	ServerID string `json:"server_id"`
	Domain   string `json:"domain"`
}

// CreateMailboxRequest holds parameters to create or update a virtual mailbox.
type CreateMailboxRequest struct {
	ServerID string `json:"server_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
	QuotaMB  int    `json:"quota_mb"`
}

// DeleteMailboxRequest holds parameters to delete a mailbox.
type DeleteMailboxRequest struct {
	ServerID string `json:"server_id"`
	Email    string `json:"email"`
}

// MailAliasRequest holds parameters to add or delete a virtual alias.
type MailAliasRequest struct {
	ServerID    string `json:"server_id"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
}
