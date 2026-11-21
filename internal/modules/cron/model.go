package cron

// CronJob represents a scheduled task in the system.
type CronJob struct {
	ID         string `json:"id"`
	Expression string `json:"expression"` // e.g. "0 * * * *" or "*/15 * * * *"
	Command    string `json:"command"`    // e.g. "php /var/www/app/artisan schedule:run"
	Enabled    bool   `json:"enabled"`
	Comment    string `json:"comment"`
	User       string `json:"user"`             // e.g. "root" or "www-data"
	Source     string `json:"source,omitempty"` // e.g. "crontab", "/etc/crontab", "/etc/cron.d/certbot"
}

// SaveCronJobRequest for creating or updating a cron job.
type SaveCronJobRequest struct {
	ServerID string  `json:"server_id"`
	Job      CronJob `json:"job"`
}

// DeleteCronJobRequest for deleting a job.
type DeleteCronJobRequest struct {
	ServerID string `json:"server_id"`
	JobID    string `json:"job_id"`
	User     string `json:"user"`
}

// ToggleCronJobRequest for enabling or disabling a job.
type ToggleCronJobRequest struct {
	ServerID string `json:"server_id"`
	JobID    string `json:"job_id"`
	Enabled  bool   `json:"enabled"`
	User     string `json:"user"`
}
