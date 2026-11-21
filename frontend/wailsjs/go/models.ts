export namespace agents {
	
	export class AgentFinding {
	    id: string;
	    agent_id: string;
	    server_id: string;
	    severity: string;
	    title: string;
	    details: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new AgentFinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.agent_id = source["agent_id"];
	        this.server_id = source["server_id"];
	        this.severity = source["severity"];
	        this.title = source["title"];
	        this.details = source["details"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AgentStatus {
	    id: string;
	    name: string;
	    description: string;
	    enabled: boolean;
	    interval_seconds: number;
	    // Go type: time
	    last_run_at?: any;
	    // Go type: time
	    next_run_at?: any;
	    last_status: string;
	    last_error?: string;
	    findings_count: number;
	    recent_findings: AgentFinding[];
	
	    static createFrom(source: any = {}) {
	        return new AgentStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.enabled = source["enabled"];
	        this.interval_seconds = source["interval_seconds"];
	        this.last_run_at = this.convertValues(source["last_run_at"], null);
	        this.next_run_at = this.convertValues(source["next_run_at"], null);
	        this.last_status = source["last_status"];
	        this.last_error = source["last_error"];
	        this.findings_count = source["findings_count"];
	        this.recent_findings = this.convertValues(source["recent_findings"], AgentFinding);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace apps {
	
	export class AppRuntimeInfo {
	    node_installed: boolean;
	    node_version?: string;
	    npm_installed: boolean;
	    npm_version?: string;
	    python_installed: boolean;
	    python_version?: string;
	    go_installed: boolean;
	    go_version?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppRuntimeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node_installed = source["node_installed"];
	        this.node_version = source["node_version"];
	        this.npm_installed = source["npm_installed"];
	        this.npm_version = source["npm_version"];
	        this.python_installed = source["python_installed"];
	        this.python_version = source["python_version"];
	        this.go_installed = source["go_installed"];
	        this.go_version = source["go_version"];
	    }
	}
	export class AppIssue {
	    severity: string;
	    code: string;
	    message: string;
	    fixable: boolean;
	    fix_action?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppIssue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.code = source["code"];
	        this.message = source["message"];
	        this.fixable = source["fixable"];
	        this.fix_action = source["fix_action"];
	    }
	}
	export class AppInspectResult {
	    site_id: string;
	    domain: string;
	    framework: string;
	    framework_version?: string;
	    required_php?: string;
	    has_artisan: boolean;
	    has_composer: boolean;
	    has_wp_config: boolean;
	    has_console: boolean;
	    has_package_json: boolean;
	    storage_writable: boolean;
	    bootstrap_cache_writable: boolean;
	    has_env_file: boolean;
	    has_app_key: boolean;
	    scheduler_active: boolean;
	    queue_worker_status: string;
	    queue_worker_unit?: string;
	    issues: AppIssue[];
	    available_logs: string[];
	    runtimes: AppRuntimeInfo;
	
	    static createFrom(source: any = {}) {
	        return new AppInspectResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.site_id = source["site_id"];
	        this.domain = source["domain"];
	        this.framework = source["framework"];
	        this.framework_version = source["framework_version"];
	        this.required_php = source["required_php"];
	        this.has_artisan = source["has_artisan"];
	        this.has_composer = source["has_composer"];
	        this.has_wp_config = source["has_wp_config"];
	        this.has_console = source["has_console"];
	        this.has_package_json = source["has_package_json"];
	        this.storage_writable = source["storage_writable"];
	        this.bootstrap_cache_writable = source["bootstrap_cache_writable"];
	        this.has_env_file = source["has_env_file"];
	        this.has_app_key = source["has_app_key"];
	        this.scheduler_active = source["scheduler_active"];
	        this.queue_worker_status = source["queue_worker_status"];
	        this.queue_worker_unit = source["queue_worker_unit"];
	        this.issues = this.convertValues(source["issues"], AppIssue);
	        this.available_logs = source["available_logs"];
	        this.runtimes = this.convertValues(source["runtimes"], AppRuntimeInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class ArtisanCommandResult {
	    command: string;
	    output: string;
	    success: boolean;
	    duration_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new ArtisanCommandResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.output = source["output"];
	        this.success = source["success"];
	        this.duration_ms = source["duration_ms"];
	    }
	}
	export class EnvFileResult {
	    site_id: string;
	    path: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvFileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.site_id = source["site_id"];
	        this.path = source["path"];
	        this.content = source["content"];
	    }
	}
	export class QueueWorkerConfig {
	    site_id: string;
	    connection?: string;
	    queue?: string;
	    sleep?: number;
	    tries?: number;
	    timeout?: number;
	
	    static createFrom(source: any = {}) {
	        return new QueueWorkerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.site_id = source["site_id"];
	        this.connection = source["connection"];
	        this.queue = source["queue"];
	        this.sleep = source["sleep"];
	        this.tries = source["tries"];
	        this.timeout = source["timeout"];
	    }
	}

}

export namespace backup {
	
	export class ArchiveInspectionResult {
	    file_path: string;
	    archive_type: string;
	    size_bytes: number;
	    total_entries: number;
	    sample_entries: string[];
	    has_system_paths: boolean;
	    dangerous_paths: string[];
	    suggested_root: string;
	    contains_db_dump: boolean;
	    db_dump_name?: string;
	    integrity_passed: boolean;
	    // Go type: time
	    inspected_at: any;
	    error_message?: string;
	
	    static createFrom(source: any = {}) {
	        return new ArchiveInspectionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_path = source["file_path"];
	        this.archive_type = source["archive_type"];
	        this.size_bytes = source["size_bytes"];
	        this.total_entries = source["total_entries"];
	        this.sample_entries = source["sample_entries"];
	        this.has_system_paths = source["has_system_paths"];
	        this.dangerous_paths = source["dangerous_paths"];
	        this.suggested_root = source["suggested_root"];
	        this.contains_db_dump = source["contains_db_dump"];
	        this.db_dump_name = source["db_dump_name"];
	        this.integrity_passed = source["integrity_passed"];
	        this.inspected_at = this.convertValues(source["inspected_at"], null);
	        this.error_message = source["error_message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CreateBackupRequest {
	    server_id: string;
	    backup_type: string;
	    target_name: string;
	    destination: string;
	    target_path: string;
	    retention_days: number;
	    include_database: boolean;
	    database_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateBackupRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.backup_type = source["backup_type"];
	        this.target_name = source["target_name"];
	        this.destination = source["destination"];
	        this.target_path = source["target_path"];
	        this.retention_days = source["retention_days"];
	        this.include_database = source["include_database"];
	        this.database_name = source["database_name"];
	    }
	}
	export class DiscoverBackupsResult {
	    scanned_paths: string[];
	    found: number;
	    already_known: number;
	    registered: db.BackupRecord[];
	    ignored_unknown: number;
	
	    static createFrom(source: any = {}) {
	        return new DiscoverBackupsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scanned_paths = source["scanned_paths"];
	        this.found = source["found"];
	        this.already_known = source["already_known"];
	        this.registered = this.convertValues(source["registered"], db.BackupRecord);
	        this.ignored_unknown = source["ignored_unknown"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RestoreBackupRequest {
	    server_id: string;
	    backup_id: string;
	    file_path: string;
	    target_type: string;
	    target_name: string;
	    target_directory?: string;
	    allow_system_paths: boolean;
	    dry_run: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RestoreBackupRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.backup_id = source["backup_id"];
	        this.file_path = source["file_path"];
	        this.target_type = source["target_type"];
	        this.target_name = source["target_name"];
	        this.target_directory = source["target_directory"];
	        this.allow_system_paths = source["allow_system_paths"];
	        this.dry_run = source["dry_run"];
	    }
	}

}

export namespace chat {
	
	export class ProviderConfigDTO {
	    id: string;
	    provider_type: string;
	    name: string;
	    base_url: string;
	    has_api_key: boolean;
	    is_enabled: boolean;
	    is_default: boolean;
	    default_model: string;
	    system_prompt: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new ProviderConfigDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.provider_type = source["provider_type"];
	        this.name = source["name"];
	        this.base_url = source["base_url"];
	        this.has_api_key = source["has_api_key"];
	        this.is_enabled = source["is_enabled"];
	        this.is_default = source["is_default"];
	        this.default_model = source["default_model"];
	        this.system_prompt = source["system_prompt"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProviderConfigInput {
	    id: string;
	    provider_type: string;
	    name: string;
	    base_url: string;
	    api_key?: string;
	    is_enabled: boolean;
	    is_default: boolean;
	    default_model: string;
	    system_prompt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderConfigInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.provider_type = source["provider_type"];
	        this.name = source["name"];
	        this.base_url = source["base_url"];
	        this.api_key = source["api_key"];
	        this.is_enabled = source["is_enabled"];
	        this.is_default = source["is_default"];
	        this.default_model = source["default_model"];
	        this.system_prompt = source["system_prompt"];
	    }
	}

}

export namespace cron {
	
	export class CronJob {
	    id: string;
	    expression: string;
	    command: string;
	    enabled: boolean;
	    comment: string;
	    user: string;
	    source?: string;
	
	    static createFrom(source: any = {}) {
	        return new CronJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.expression = source["expression"];
	        this.command = source["command"];
	        this.enabled = source["enabled"];
	        this.comment = source["comment"];
	        this.user = source["user"];
	        this.source = source["source"];
	    }
	}
	export class DeleteCronJobRequest {
	    server_id: string;
	    job_id: string;
	    user: string;
	
	    static createFrom(source: any = {}) {
	        return new DeleteCronJobRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.job_id = source["job_id"];
	        this.user = source["user"];
	    }
	}
	export class SaveCronJobRequest {
	    server_id: string;
	    job: CronJob;
	
	    static createFrom(source: any = {}) {
	        return new SaveCronJobRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.job = this.convertValues(source["job"], CronJob);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ToggleCronJobRequest {
	    server_id: string;
	    job_id: string;
	    enabled: boolean;
	    user: string;
	
	    static createFrom(source: any = {}) {
	        return new ToggleCronJobRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.job_id = source["job_id"];
	        this.enabled = source["enabled"];
	        this.user = source["user"];
	    }
	}

}

export namespace database {
	
	export class BackupDatabaseRequest {
	    server_id: string;
	    engine: string;
	    database_name: string;
	    target_path: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupDatabaseRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.engine = source["engine"];
	        this.database_name = source["database_name"];
	        this.target_path = source["target_path"];
	    }
	}
	export class CreateDatabaseRequest {
	    server_id: string;
	    engine: string;
	    name: string;
	    charset: string;
	    collation: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateDatabaseRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.engine = source["engine"];
	        this.name = source["name"];
	        this.charset = source["charset"];
	        this.collation = source["collation"];
	    }
	}
	export class CreateUserRequest {
	    server_id: string;
	    engine: string;
	    username: string;
	    password: string;
	    host: string;
	    databases: string[];
	
	    static createFrom(source: any = {}) {
	        return new CreateUserRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.engine = source["engine"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.host = source["host"];
	        this.databases = source["databases"];
	    }
	}
	export class DBInstallPlan {
	    engine: string;
	    variant: string;
	    os_family: string;
	    os_distro: string;
	    os_compatible: boolean;
	    packages_to_install: string[];
	    service_to_start: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DBInstallPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.variant = source["variant"];
	        this.os_family = source["os_family"];
	        this.os_distro = source["os_distro"];
	        this.os_compatible = source["os_compatible"];
	        this.packages_to_install = source["packages_to_install"];
	        this.service_to_start = source["service_to_start"];
	        this.message = source["message"];
	    }
	}
	export class DatabaseInstanceInfo {
	    engine: string;
	    display_name: string;
	    installed: boolean;
	    version: string;
	    service_status: string;
	    port: number;
	    memory_bytes: number;
	    db_count: number;
	    config_path: string;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseInstanceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.display_name = source["display_name"];
	        this.installed = source["installed"];
	        this.version = source["version"];
	        this.service_status = source["service_status"];
	        this.port = source["port"];
	        this.memory_bytes = source["memory_bytes"];
	        this.db_count = source["db_count"];
	        this.config_path = source["config_path"];
	    }
	}
	export class DatabaseItem {
	    name: string;
	    engine: string;
	    size_bytes: number;
	    tables_count: number;
	    charset: string;
	    collation: string;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.engine = source["engine"];
	        this.size_bytes = source["size_bytes"];
	        this.tables_count = source["tables_count"];
	        this.charset = source["charset"];
	        this.collation = source["collation"];
	    }
	}
	export class DatabaseUser {
	    username: string;
	    host: string;
	    engine: string;
	    databases: string[];
	
	    static createFrom(source: any = {}) {
	        return new DatabaseUser(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.host = source["host"];
	        this.engine = source["engine"];
	        this.databases = source["databases"];
	    }
	}
	export class RestoreDatabaseRequest {
	    server_id: string;
	    engine: string;
	    database_name: string;
	    dump_file_path: string;
	
	    static createFrom(source: any = {}) {
	        return new RestoreDatabaseRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.engine = source["engine"];
	        this.database_name = source["database_name"];
	        this.dump_file_path = source["dump_file_path"];
	    }
	}

}

export namespace db {
	
	export class AIConversationRecord {
	    id: string;
	    title: string;
	    server_id?: string;
	    system_prompt: string;
	    selected_provider: string;
	    selected_model: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new AIConversationRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.server_id = source["server_id"];
	        this.system_prompt = source["system_prompt"];
	        this.selected_provider = source["selected_provider"];
	        this.selected_model = source["selected_model"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AIMemoryFactRecord {
	    id: string;
	    server_id?: string;
	    category: string;
	    fact_key: string;
	    fact_value: string;
	    source: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new AIMemoryFactRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server_id = source["server_id"];
	        this.category = source["category"];
	        this.fact_key = source["fact_key"];
	        this.fact_value = source["fact_value"];
	        this.source = source["source"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AIMessageRecord {
	    id: string;
	    conversation_id: string;
	    role: string;
	    content: string;
	    tool_calls_json: string;
	    tool_call_id: string;
	    prompt_tokens: number;
	    completion_tokens: number;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new AIMessageRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.conversation_id = source["conversation_id"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.tool_calls_json = source["tool_calls_json"];
	        this.tool_call_id = source["tool_call_id"];
	        this.prompt_tokens = source["prompt_tokens"];
	        this.completion_tokens = source["completion_tokens"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BackupRecord {
	    id: string;
	    server_id: string;
	    backup_type: string;
	    target_name: string;
	    destination: string;
	    file_path: string;
	    size_bytes: number;
	    status: string;
	    offsite: boolean;
	    // Go type: time
	    downloaded_at?: any;
	    verified: boolean;
	    checksum: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new BackupRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server_id = source["server_id"];
	        this.backup_type = source["backup_type"];
	        this.target_name = source["target_name"];
	        this.destination = source["destination"];
	        this.file_path = source["file_path"];
	        this.size_bytes = source["size_bytes"];
	        this.status = source["status"];
	        this.offsite = source["offsite"];
	        this.downloaded_at = this.convertValues(source["downloaded_at"], null);
	        this.verified = source["verified"];
	        this.checksum = source["checksum"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DiagnosticsRecord {
	    server_id: string;
	    os_name: string;
	    os_version: string;
	    os_family: string;
	    kernel_version: string;
	    arch: string;
	    cpu_cores: number;
	    cpu_model: string;
	    ram_total_bytes: number;
	    disk_total_bytes: number;
	    disk_used_bytes: number;
	    uptime_seconds: number;
	    ssh_version: string;
	    installed_webservers: string[];
	    installed_php_versions: string[];
	    installed_db_versions: string[];
	    docker_version: string;
	    // Go type: time
	    diagnosed_at: any;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticsRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.os_name = source["os_name"];
	        this.os_version = source["os_version"];
	        this.os_family = source["os_family"];
	        this.kernel_version = source["kernel_version"];
	        this.arch = source["arch"];
	        this.cpu_cores = source["cpu_cores"];
	        this.cpu_model = source["cpu_model"];
	        this.ram_total_bytes = source["ram_total_bytes"];
	        this.disk_total_bytes = source["disk_total_bytes"];
	        this.disk_used_bytes = source["disk_used_bytes"];
	        this.uptime_seconds = source["uptime_seconds"];
	        this.ssh_version = source["ssh_version"];
	        this.installed_webservers = source["installed_webservers"];
	        this.installed_php_versions = source["installed_php_versions"];
	        this.installed_db_versions = source["installed_db_versions"];
	        this.docker_version = source["docker_version"];
	        this.diagnosed_at = this.convertValues(source["diagnosed_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProvisioningRunRecord {
	    id: string;
	    server_id: string;
	    blueprint_id: string;
	    profile: string;
	    status: string;
	    plan_json: string;
	    result_json: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    completed_at?: any;
	
	    static createFrom(source: any = {}) {
	        return new ProvisioningRunRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server_id = source["server_id"];
	        this.blueprint_id = source["blueprint_id"];
	        this.profile = source["profile"];
	        this.status = source["status"];
	        this.plan_json = source["plan_json"];
	        this.result_json = source["result_json"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.completed_at = this.convertValues(source["completed_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SSLCertificateRecord {
	    id: string;
	    server_id: string;
	    site_id: string;
	    primary_domain: string;
	    san_domains: string[];
	    cert_path: string;
	    key_path: string;
	    issuer: string;
	    // Go type: time
	    valid_from?: any;
	    // Go type: time
	    valid_to?: any;
	    days_remaining: number;
	    auto_renew: boolean;
	    status: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new SSLCertificateRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server_id = source["server_id"];
	        this.site_id = source["site_id"];
	        this.primary_domain = source["primary_domain"];
	        this.san_domains = source["san_domains"];
	        this.cert_path = source["cert_path"];
	        this.key_path = source["key_path"];
	        this.issuer = source["issuer"];
	        this.valid_from = this.convertValues(source["valid_from"], null);
	        this.valid_to = this.convertValues(source["valid_to"], null);
	        this.days_remaining = source["days_remaining"];
	        this.auto_renew = source["auto_renew"];
	        this.status = source["status"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ServerBlueprintRecord {
	    id: string;
	    name: string;
	    description: string;
	    profile: string;
	    spec_json: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new ServerBlueprintRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.profile = source["profile"];
	        this.spec_json = source["spec_json"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ServerRecord {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    username: string;
	    auth_type: string;
	    ssh_host_key: string;
	    group_name: string;
	    tags: string[];
	    is_favorite: boolean;
	    status: string;
	    mode: string;
	    // Go type: time
	    last_connected_at?: any;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new ServerRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.auth_type = source["auth_type"];
	        this.ssh_host_key = source["ssh_host_key"];
	        this.group_name = source["group_name"];
	        this.tags = source["tags"];
	        this.is_favorite = source["is_favorite"];
	        this.status = source["status"];
	        this.mode = source["mode"];
	        this.last_connected_at = this.convertValues(source["last_connected_at"], null);
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SiteConfigBackupRecord {
	    id: string;
	    server_id: string;
	    site_id: string;
	    file_path: string;
	    content_hash: string;
	    content: string;
	    remote_backup_path: string;
	    task_id: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new SiteConfigBackupRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server_id = source["server_id"];
	        this.site_id = source["site_id"];
	        this.file_path = source["file_path"];
	        this.content_hash = source["content_hash"];
	        this.content = source["content"];
	        this.remote_backup_path = source["remote_backup_path"];
	        this.task_id = source["task_id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SiteRecord {
	    id: string;
	    server_id: string;
	    domain: string;
	    aliases: string[];
	    document_root: string;
	    site_type: string;
	    php_version: string;
	    fpm_pool: string;
	    web_server: string;
	    ssl_enabled: boolean;
	    ssl_provider: string;
	    status: string;
	    origin: string;
	    vhost_path: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new SiteRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server_id = source["server_id"];
	        this.domain = source["domain"];
	        this.aliases = source["aliases"];
	        this.document_root = source["document_root"];
	        this.site_type = source["site_type"];
	        this.php_version = source["php_version"];
	        this.fpm_pool = source["fpm_pool"];
	        this.web_server = source["web_server"];
	        this.ssl_enabled = source["ssl_enabled"];
	        this.ssl_provider = source["ssl_provider"];
	        this.status = source["status"];
	        this.origin = source["origin"];
	        this.vhost_path = source["vhost_path"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TaskLogRecord {
	    id: number;
	    task_id: string;
	    level: string;
	    message: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new TaskLogRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.task_id = source["task_id"];
	        this.level = source["level"];
	        this.message = source["message"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TaskRecord {
	    id: string;
	    server_id: string;
	    operation_type: string;
	    title: string;
	    status: string;
	    current_step: string;
	    progress: number;
	    payload: string;
	    result: string;
	    error_message: string;
	    // Go type: time
	    started_at?: any;
	    // Go type: time
	    completed_at?: any;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new TaskRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server_id = source["server_id"];
	        this.operation_type = source["operation_type"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.current_step = source["current_step"];
	        this.progress = source["progress"];
	        this.payload = source["payload"];
	        this.result = source["result"];
	        this.error_message = source["error_message"];
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.completed_at = this.convertValues(source["completed_at"], null);
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace docker {
	
	export class ComposeActionRequest {
	    server_id: string;
	    working_dir: string;
	    action: string;
	    content?: string;
	
	    static createFrom(source: any = {}) {
	        return new ComposeActionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.working_dir = source["working_dir"];
	        this.action = source["action"];
	        this.content = source["content"];
	    }
	}
	export class ComposeProject {
	    name: string;
	    status: string;
	    config_file: string;
	    working_dir: string;
	    containers: number;
	
	    static createFrom(source: any = {}) {
	        return new ComposeProject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.config_file = source["config_file"];
	        this.working_dir = source["working_dir"];
	        this.containers = source["containers"];
	    }
	}
	export class ContainerActionRequest {
	    server_id: string;
	    container_id: string;
	    action: string;
	    force: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContainerActionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.container_id = source["container_id"];
	        this.action = source["action"];
	        this.force = source["force"];
	    }
	}
	export class ContainerItem {
	    id: string;
	    names: string;
	    image: string;
	    command: string;
	    created_at: string;
	    status: string;
	    state: string;
	    ports: string;
	    size: string;
	    cpu_percent: string;
	    mem_usage: string;
	
	    static createFrom(source: any = {}) {
	        return new ContainerItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.names = source["names"];
	        this.image = source["image"];
	        this.command = source["command"];
	        this.created_at = source["created_at"];
	        this.status = source["status"];
	        this.state = source["state"];
	        this.ports = source["ports"];
	        this.size = source["size"];
	        this.cpu_percent = source["cpu_percent"];
	        this.mem_usage = source["mem_usage"];
	    }
	}
	export class DockerPruneRequest {
	    server_id: string;
	    prune_type: string;
	    prune_volumes: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DockerPruneRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.prune_type = source["prune_type"];
	        this.prune_volumes = source["prune_volumes"];
	    }
	}
	export class DockerStatus {
	    installed: boolean;
	    version: string;
	    compose_version: string;
	    service_status: string;
	    containers_count: number;
	    running_count: number;
	    images_count: number;
	    volumes_count: number;
	
	    static createFrom(source: any = {}) {
	        return new DockerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.version = source["version"];
	        this.compose_version = source["compose_version"];
	        this.service_status = source["service_status"];
	        this.containers_count = source["containers_count"];
	        this.running_count = source["running_count"];
	        this.images_count = source["images_count"];
	        this.volumes_count = source["volumes_count"];
	    }
	}
	export class ImageItem {
	    id: string;
	    repository: string;
	    tag: string;
	    size: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new ImageItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.repository = source["repository"];
	        this.tag = source["tag"];
	        this.size = source["size"];
	        this.created_at = source["created_at"];
	    }
	}
	export class NetworkItem {
	    id: string;
	    name: string;
	    driver: string;
	    scope: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.driver = source["driver"];
	        this.scope = source["scope"];
	    }
	}
	export class PullImageRequest {
	    server_id: string;
	    image: string;
	
	    static createFrom(source: any = {}) {
	        return new PullImageRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.image = source["image"];
	    }
	}
	export class VolumeItem {
	    name: string;
	    driver: string;
	    scope: string;
	    size_bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new VolumeItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.driver = source["driver"];
	        this.scope = source["scope"];
	        this.size_bytes = source["size_bytes"];
	    }
	}

}

export namespace domain {
	
	export class ModelSpec {
	    id: string;
	    name: string;
	    provider: string;
	    context_window: number;
	    supports_tools: boolean;
	    supports_streaming: boolean;
	    is_local: boolean;
	    input_cost_per_m?: number;
	    output_cost_per_m?: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.context_window = source["context_window"];
	        this.supports_tools = source["supports_tools"];
	        this.supports_streaming = source["supports_streaming"];
	        this.is_local = source["is_local"];
	        this.input_cost_per_m = source["input_cost_per_m"];
	        this.output_cost_per_m = source["output_cost_per_m"];
	    }
	}
	export class ToolDefinition {
	    name: string;
	    description: string;
	    parameters: Record<string, any>;
	    risk_level: string;
	    is_mutating: boolean;
	    approval_via_task?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ToolDefinition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.parameters = source["parameters"];
	        this.risk_level = source["risk_level"];
	        this.is_mutating = source["is_mutating"];
	        this.approval_via_task = source["approval_via_task"];
	    }
	}

}

export namespace files {
	
	export class ChmodRequest {
	    server_id: string;
	    path: string;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new ChmodRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.path = source["path"];
	        this.mode = source["mode"];
	    }
	}
	export class CleanupCategory {
	    id: string;
	    title: string;
	    description: string;
	    size_bytes: number;
	    human_size: string;
	    safe: boolean;
	    available: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CleanupCategory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.size_bytes = source["size_bytes"];
	        this.human_size = source["human_size"];
	        this.safe = source["safe"];
	        this.available = source["available"];
	    }
	}
	export class CreateEntryRequest {
	    server_id: string;
	    path: string;
	    is_dir: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CreateEntryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.path = source["path"];
	        this.is_dir = source["is_dir"];
	    }
	}
	export class DeleteEntryRequest {
	    server_id: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new DeleteEntryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.path = source["path"];
	    }
	}
	export class FileEntry {
	    name: string;
	    path: string;
	    size: number;
	    human_size: string;
	    is_dir: boolean;
	    permissions: string;
	    owner: string;
	    group: string;
	    // Go type: time
	    mod_time: any;
	    extension: string;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.human_size = source["human_size"];
	        this.is_dir = source["is_dir"];
	        this.permissions = source["permissions"];
	        this.owner = source["owner"];
	        this.group = source["group"];
	        this.mod_time = this.convertValues(source["mod_time"], null);
	        this.extension = source["extension"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DirectoryListing {
	    current_path: string;
	    parent_path: string;
	    total_items: number;
	    entries: FileEntry[];
	
	    static createFrom(source: any = {}) {
	        return new DirectoryListing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current_path = source["current_path"];
	        this.parent_path = source["parent_path"];
	        this.total_items = source["total_items"];
	        this.entries = this.convertValues(source["entries"], FileEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LargestFileItem {
	    path: string;
	    name: string;
	    size_bytes: number;
	    human_size: string;
	    // Go type: time
	    mod_time: any;
	
	    static createFrom(source: any = {}) {
	        return new LargestFileItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size_bytes = source["size_bytes"];
	        this.human_size = source["human_size"];
	        this.mod_time = this.convertValues(source["mod_time"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DiskUsageItem {
	    path: string;
	    name: string;
	    size_bytes: number;
	    human_size: string;
	    is_dir: boolean;
	    percent: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskUsageItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size_bytes = source["size_bytes"];
	        this.human_size = source["human_size"];
	        this.is_dir = source["is_dir"];
	        this.percent = source["percent"];
	    }
	}
	export class DiskPartition {
	    filesystem: string;
	    mount_point: string;
	    total_bytes: number;
	    used_bytes: number;
	    free_bytes: number;
	    used_percent: number;
	    human_total: string;
	    human_used: string;
	    human_free: string;
	
	    static createFrom(source: any = {}) {
	        return new DiskPartition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filesystem = source["filesystem"];
	        this.mount_point = source["mount_point"];
	        this.total_bytes = source["total_bytes"];
	        this.used_bytes = source["used_bytes"];
	        this.free_bytes = source["free_bytes"];
	        this.used_percent = source["used_percent"];
	        this.human_total = source["human_total"];
	        this.human_used = source["human_used"];
	        this.human_free = source["human_free"];
	    }
	}
	export class DiskAnalysisReport {
	    server_id: string;
	    analyzed_path: string;
	    partitions: DiskPartition[];
	    top_directories: DiskUsageItem[];
	    largest_files: LargestFileItem[];
	    cleanup_options: CleanupCategory[];
	    total_path_bytes: number;
	    human_total_path: string;
	    // Go type: time
	    analyzed_at: any;
	
	    static createFrom(source: any = {}) {
	        return new DiskAnalysisReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.analyzed_path = source["analyzed_path"];
	        this.partitions = this.convertValues(source["partitions"], DiskPartition);
	        this.top_directories = this.convertValues(source["top_directories"], DiskUsageItem);
	        this.largest_files = this.convertValues(source["largest_files"], LargestFileItem);
	        this.cleanup_options = this.convertValues(source["cleanup_options"], CleanupCategory);
	        this.total_path_bytes = source["total_path_bytes"];
	        this.human_total_path = source["human_total_path"];
	        this.analyzed_at = this.convertValues(source["analyzed_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class FileContentResult {
	    path: string;
	    content: string;
	    size_bytes: number;
	    is_binary: boolean;
	    permissions: string;
	
	    static createFrom(source: any = {}) {
	        return new FileContentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.content = source["content"];
	        this.size_bytes = source["size_bytes"];
	        this.is_binary = source["is_binary"];
	        this.permissions = source["permissions"];
	    }
	}
	
	
	export class RenameEntryRequest {
	    server_id: string;
	    old_path: string;
	    new_path: string;
	
	    static createFrom(source: any = {}) {
	        return new RenameEntryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.old_path = source["old_path"];
	        this.new_path = source["new_path"];
	    }
	}
	export class SaveFileRequest {
	    server_id: string;
	    path: string;
	    content: string;
	    create_backup: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SaveFileRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.path = source["path"];
	        this.content = source["content"];
	        this.create_backup = source["create_backup"];
	    }
	}

}

export namespace inventory {
	
	export class RepoDiagnostic {
	    url: string;
	    codename: string;
	    component: string;
	    file_path: string;
	    is_mismatch: boolean;
	    expected_codename: string;
	    missing_keys: string[];
	    is_expired: boolean;
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoDiagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.codename = source["codename"];
	        this.component = source["component"];
	        this.file_path = source["file_path"];
	        this.is_mismatch = source["is_mismatch"];
	        this.expected_codename = source["expected_codename"];
	        this.missing_keys = source["missing_keys"];
	        this.is_expired = source["is_expired"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class APTDiagnostics {
	    sources: RepoDiagnostic[];
	    missing_gpg_keys: string[];
	    codename_mismatches: string[];
	    broken_packages: string[];
	    duplicate_sources: string[];
	    has_errors: boolean;
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new APTDiagnostics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sources = this.convertValues(source["sources"], RepoDiagnostic);
	        this.missing_gpg_keys = source["missing_gpg_keys"];
	        this.codename_mismatches = source["codename_mismatches"];
	        this.broken_packages = source["broken_packages"];
	        this.duplicate_sources = source["duplicate_sources"];
	        this.has_errors = source["has_errors"];
	        this.summary = source["summary"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ContainerInfo {
	    id: string;
	    names: string;
	    image: string;
	    status: string;
	    ports: string;
	
	    static createFrom(source: any = {}) {
	        return new ContainerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.names = source["names"];
	        this.image = source["image"];
	        this.status = source["status"];
	        this.ports = source["ports"];
	    }
	}
	export class CronEntry {
	    schedule: string;
	    command: string;
	    user: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new CronEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schedule = source["schedule"];
	        this.command = source["command"];
	        this.user = source["user"];
	        this.source = source["source"];
	    }
	}
	export class CurrentUserDetail {
	    username: string;
	    uid: number;
	    gid: number;
	    groups: string[];
	    has_sudo: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CurrentUserDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.uid = source["uid"];
	        this.gid = source["gid"];
	        this.groups = source["groups"];
	        this.has_sudo = source["has_sudo"];
	    }
	}
	export class DatabaseFacts {
	    engine: string;
	    version: string;
	    databases: string[];
	
	    static createFrom(source: any = {}) {
	        return new DatabaseFacts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.version = source["version"];
	        this.databases = source["databases"];
	    }
	}
	export class DatabaseInstance {
	    engine: string;
	    version: string;
	    binary_path: string;
	    config_path: string;
	    status: string;
	    ports: number[];
	    sockets: string[];
	    systemd_unit: string;
	    install_type: string;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseInstance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.version = source["version"];
	        this.binary_path = source["binary_path"];
	        this.config_path = source["config_path"];
	        this.status = source["status"];
	        this.ports = source["ports"];
	        this.sockets = source["sockets"];
	        this.systemd_unit = source["systemd_unit"];
	        this.install_type = source["install_type"];
	    }
	}
	export class FpmPoolFact {
	    pool_name: string;
	    listen: string;
	    user: string;
	    group: string;
	    pm_mode: string;
	    max_children: number;
	    config_path: string;
	
	    static createFrom(source: any = {}) {
	        return new FpmPoolFact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pool_name = source["pool_name"];
	        this.listen = source["listen"];
	        this.user = source["user"];
	        this.group = source["group"];
	        this.pm_mode = source["pm_mode"];
	        this.max_children = source["max_children"];
	        this.config_path = source["config_path"];
	    }
	}
	export class ListeningSocket {
	    proto: string;
	    local: string;
	    port: number;
	    process: string;
	    pid: number;
	
	    static createFrom(source: any = {}) {
	        return new ListeningSocket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proto = source["proto"];
	        this.local = source["local"];
	        this.port = source["port"];
	        this.process = source["process"];
	        this.pid = source["pid"];
	    }
	}
	export class MountPoint {
	    filesystem: string;
	    mount: string;
	    type: string;
	    total_bytes: number;
	    free_bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new MountPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filesystem = source["filesystem"];
	        this.mount = source["mount"];
	        this.type = source["type"];
	        this.total_bytes = source["total_bytes"];
	        this.free_bytes = source["free_bytes"];
	    }
	}
	export class OSInfo {
	    distro: string;
	    version: string;
	    codename: string;
	    architecture: string;
	    kernel: string;
	    init_system: string;
	    package_manager: string;
	    current_user: CurrentUserDetail;
	
	    static createFrom(source: any = {}) {
	        return new OSInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.distro = source["distro"];
	        this.version = source["version"];
	        this.codename = source["codename"];
	        this.architecture = source["architecture"];
	        this.kernel = source["kernel"];
	        this.init_system = source["init_system"];
	        this.package_manager = source["package_manager"];
	        this.current_user = this.convertValues(source["current_user"], CurrentUserDetail);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class OSUser {
	    username: string;
	    uid: number;
	    gid: number;
	    home: string;
	    shell: string;
	    is_system: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OSUser(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.uid = source["uid"];
	        this.gid = source["gid"];
	        this.home = source["home"];
	        this.shell = source["shell"];
	        this.is_system = source["is_system"];
	    }
	}
	export class PHPRuntimeInstance {
	    id: string;
	    version: string;
	    major_minor: string;
	    kind: string;
	    binary_path: string;
	    status: string;
	    is_orphaned: boolean;
	    is_manageable: boolean;
	    sockets: string[];
	    tcp_listeners: string[];
	    config_dir: string;
	    ini_path: string;
	    systemd_unit: string;
	    modules: string[];
	    active_pools: FpmPoolFact[];
	    limitations: string[];
	
	    static createFrom(source: any = {}) {
	        return new PHPRuntimeInstance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.version = source["version"];
	        this.major_minor = source["major_minor"];
	        this.kind = source["kind"];
	        this.binary_path = source["binary_path"];
	        this.status = source["status"];
	        this.is_orphaned = source["is_orphaned"];
	        this.is_manageable = source["is_manageable"];
	        this.sockets = source["sockets"];
	        this.tcp_listeners = source["tcp_listeners"];
	        this.config_dir = source["config_dir"];
	        this.ini_path = source["ini_path"];
	        this.systemd_unit = source["systemd_unit"];
	        this.modules = source["modules"];
	        this.active_pools = this.convertValues(source["active_pools"], FpmPoolFact);
	        this.limitations = source["limitations"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ServerInventory {
	    distro: string;
	    family: string;
	    version: string;
	    codename: string;
	    arch: string;
	    kernel: string;
	    cpu_cores: number;
	    ram_total_mb: number;
	    ram_free_mb: number;
	    disk_total_gb: number;
	    disk_free_gb: number;
	    package_manager: string;
	    ssh_port: number;
	    listening_ports: number[];
	    installed_web: string[];
	    installed_php: string[];
	    installed_db: string[];
	    has_docker: boolean;
	    has_ufw: boolean;
	    has_firewalld: boolean;
	    has_fail2ban: boolean;
	    has_composer: boolean;
	    has_node: boolean;
	    has_git: boolean;
	    existing_sites_count: number;
	    is_already_configured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ServerInventory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.distro = source["distro"];
	        this.family = source["family"];
	        this.version = source["version"];
	        this.codename = source["codename"];
	        this.arch = source["arch"];
	        this.kernel = source["kernel"];
	        this.cpu_cores = source["cpu_cores"];
	        this.ram_total_mb = source["ram_total_mb"];
	        this.ram_free_mb = source["ram_free_mb"];
	        this.disk_total_gb = source["disk_total_gb"];
	        this.disk_free_gb = source["disk_free_gb"];
	        this.package_manager = source["package_manager"];
	        this.ssh_port = source["ssh_port"];
	        this.listening_ports = source["listening_ports"];
	        this.installed_web = source["installed_web"];
	        this.installed_php = source["installed_php"];
	        this.installed_db = source["installed_db"];
	        this.has_docker = source["has_docker"];
	        this.has_ufw = source["has_ufw"];
	        this.has_firewalld = source["has_firewalld"];
	        this.has_fail2ban = source["has_fail2ban"];
	        this.has_composer = source["has_composer"];
	        this.has_node = source["has_node"];
	        this.has_git = source["has_git"];
	        this.existing_sites_count = source["existing_sites_count"];
	        this.is_already_configured = source["is_already_configured"];
	    }
	}
	export class SiteTopologyChain {
	    domain: string;
	    vhost_path: string;
	    web_server_instance_id: string;
	    web_server_engine: string;
	    listen_ports: number[];
	    ssl_enabled: boolean;
	    ssl_cert_path: string;
	    backend_type: string;
	    backend_target: string;
	    php_version: string;
	    php_runtime_id: string;
	    confidence: string;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new SiteTopologyChain(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.domain = source["domain"];
	        this.vhost_path = source["vhost_path"];
	        this.web_server_instance_id = source["web_server_instance_id"];
	        this.web_server_engine = source["web_server_engine"];
	        this.listen_ports = source["listen_ports"];
	        this.ssl_enabled = source["ssl_enabled"];
	        this.ssl_cert_path = source["ssl_cert_path"];
	        this.backend_type = source["backend_type"];
	        this.backend_target = source["backend_target"];
	        this.php_version = source["php_version"];
	        this.php_runtime_id = source["php_runtime_id"];
	        this.confidence = source["confidence"];
	        this.warnings = source["warnings"];
	    }
	}
	export class WebServerInstance {
	    id: string;
	    engine: string;
	    name: string;
	    binary_path: string;
	    version: string;
	    install_type: string;
	    status: string;
	    pids: number[];
	    listening_ports: number[];
	    listening_sockets: string[];
	    config_path: string;
	    prefix_dir: string;
	    vhost_includes: string[];
	    systemd_unit: string;
	    systemd_overrides: string[];
	    modules: string[];
	    is_manageable: boolean;
	    test_command: string;
	    reload_command: string;
	    limitations: string[];
	
	    static createFrom(source: any = {}) {
	        return new WebServerInstance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.engine = source["engine"];
	        this.name = source["name"];
	        this.binary_path = source["binary_path"];
	        this.version = source["version"];
	        this.install_type = source["install_type"];
	        this.status = source["status"];
	        this.pids = source["pids"];
	        this.listening_ports = source["listening_ports"];
	        this.listening_sockets = source["listening_sockets"];
	        this.config_path = source["config_path"];
	        this.prefix_dir = source["prefix_dir"];
	        this.vhost_includes = source["vhost_includes"];
	        this.systemd_unit = source["systemd_unit"];
	        this.systemd_overrides = source["systemd_overrides"];
	        this.modules = source["modules"];
	        this.is_manageable = source["is_manageable"];
	        this.test_command = source["test_command"];
	        this.reload_command = source["reload_command"];
	        this.limitations = source["limitations"];
	    }
	}
	export class VerifiedLink {
	    site_domain: string;
	    vhost_path: string;
	    fpm_socket: string;
	    pool_name: string;
	    cert_path: string;
	    status: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new VerifiedLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.site_domain = source["site_domain"];
	        this.vhost_path = source["vhost_path"];
	        this.fpm_socket = source["fpm_socket"];
	        this.pool_name = source["pool_name"];
	        this.cert_path = source["cert_path"];
	        this.status = source["status"];
	        this.note = source["note"];
	    }
	}
	export class SystemUnit {
	    unit: string;
	    load: string;
	    active: string;
	    sub: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemUnit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.unit = source["unit"];
	        this.load = source["load"];
	        this.active = source["active"];
	        this.sub = source["sub"];
	        this.description = source["description"];
	    }
	}
	export class ServerInventorySnapshot {
	    id: string;
	    server_id: string;
	    units: SystemUnit[];
	    sockets: ListeningSocket[];
	    users: OSUser[];
	    mounts: MountPoint[];
	    cron_jobs: CronEntry[];
	    containers: ContainerInfo[];
	    databases: DatabaseFacts[];
	    links: VerifiedLink[];
	    os: OSInfo;
	    web_servers: WebServerInstance[];
	    php_runtimes: PHPRuntimeInstance[];
	    db_instances: DatabaseInstance[];
	    apt: APTDiagnostics;
	    site_chains: SiteTopologyChain[];
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new ServerInventorySnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server_id = source["server_id"];
	        this.units = this.convertValues(source["units"], SystemUnit);
	        this.sockets = this.convertValues(source["sockets"], ListeningSocket);
	        this.users = this.convertValues(source["users"], OSUser);
	        this.mounts = this.convertValues(source["mounts"], MountPoint);
	        this.cron_jobs = this.convertValues(source["cron_jobs"], CronEntry);
	        this.containers = this.convertValues(source["containers"], ContainerInfo);
	        this.databases = this.convertValues(source["databases"], DatabaseFacts);
	        this.links = this.convertValues(source["links"], VerifiedLink);
	        this.os = this.convertValues(source["os"], OSInfo);
	        this.web_servers = this.convertValues(source["web_servers"], WebServerInstance);
	        this.php_runtimes = this.convertValues(source["php_runtimes"], PHPRuntimeInstance);
	        this.db_instances = this.convertValues(source["db_instances"], DatabaseInstance);
	        this.apt = this.convertValues(source["apt"], APTDiagnostics);
	        this.site_chains = this.convertValues(source["site_chains"], SiteTopologyChain);
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ServerTopology {
	    os: OSInfo;
	    web_servers: WebServerInstance[];
	    php_runtimes: PHPRuntimeInstance[];
	    databases: DatabaseInstance[];
	    apt: APTDiagnostics;
	    site_chains: SiteTopologyChain[];
	
	    static createFrom(source: any = {}) {
	        return new ServerTopology(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = this.convertValues(source["os"], OSInfo);
	        this.web_servers = this.convertValues(source["web_servers"], WebServerInstance);
	        this.php_runtimes = this.convertValues(source["php_runtimes"], PHPRuntimeInstance);
	        this.databases = this.convertValues(source["databases"], DatabaseInstance);
	        this.apt = this.convertValues(source["apt"], APTDiagnostics);
	        this.site_chains = this.convertValues(source["site_chains"], SiteTopologyChain);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

export namespace logs {
	
	export class LogTailResult {
	    file_path: string;
	    lines: string[];
	    line_count: number;
	    truncated: boolean;
	    size_bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new LogTailResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_path = source["file_path"];
	        this.lines = source["lines"];
	        this.line_count = source["line_count"];
	        this.truncated = source["truncated"];
	        this.size_bytes = source["size_bytes"];
	    }
	}

}

export namespace main {
	
	export class AppSettingsInfo {
	    data_dir: string;
	    database_path: string;
	    database_size_bytes: number;
	    database_size_human: string;
	    known_hosts_path: string;
	    known_hosts_count: number;
	    salt_path: string;
	    vault_key_path: string;
	    log_path: string;
	    log_size_bytes: number;
	    log_size_human: string;
	    operating_system: string;
	    architecture: string;
	    go_version: string;
	    version: string;
	    ai_memory_fact_count: number;
	    active_workers: number;
	    db_encrypted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppSettingsInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data_dir = source["data_dir"];
	        this.database_path = source["database_path"];
	        this.database_size_bytes = source["database_size_bytes"];
	        this.database_size_human = source["database_size_human"];
	        this.known_hosts_path = source["known_hosts_path"];
	        this.known_hosts_count = source["known_hosts_count"];
	        this.salt_path = source["salt_path"];
	        this.vault_key_path = source["vault_key_path"];
	        this.log_path = source["log_path"];
	        this.log_size_bytes = source["log_size_bytes"];
	        this.log_size_human = source["log_size_human"];
	        this.operating_system = source["operating_system"];
	        this.architecture = source["architecture"];
	        this.go_version = source["go_version"];
	        this.version = source["version"];
	        this.ai_memory_fact_count = source["ai_memory_fact_count"];
	        this.active_workers = source["active_workers"];
	        this.db_encrypted = source["db_encrypted"];
	    }
	}
	export class BootStateInfo {
	    mode: string;
	    db_locked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BootStateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.db_locked = source["db_locked"];
	    }
	}
	export class VaultStatusInfo {
	    has_master_password: boolean;
	    is_locked: boolean;
	    auto_lock_minutes: number;
	
	    static createFrom(source: any = {}) {
	        return new VaultStatusInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.has_master_password = source["has_master_password"];
	        this.is_locked = source["is_locked"];
	        this.auto_lock_minutes = source["auto_lock_minutes"];
	    }
	}

}

export namespace models {
	
	export class ContainersSpec {
	    docker: boolean;
	    compose: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContainersSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.docker = source["docker"];
	        this.compose = source["compose"];
	    }
	}
	export class ToolsSpec {
	    composer: boolean;
	    nodejs: boolean;
	    git: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ToolsSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.composer = source["composer"];
	        this.nodejs = source["nodejs"];
	        this.git = source["git"];
	    }
	}
	export class SSLSpec {
	    provider: string;
	    email?: string;
	
	    static createFrom(source: any = {}) {
	        return new SSLSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.email = source["email"];
	    }
	}
	export class SecuritySpec {
	    firewall: string;
	    fail2ban: boolean;
	    ssh_harden: boolean;
	    disable_root: boolean;
	    allow_ssh_port: number;
	
	    static createFrom(source: any = {}) {
	        return new SecuritySpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.firewall = source["firewall"];
	        this.fail2ban = source["fail2ban"];
	        this.ssh_harden = source["ssh_harden"];
	        this.disable_root = source["disable_root"];
	        this.allow_ssh_port = source["allow_ssh_port"];
	    }
	}
	export class CacheSpec {
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new CacheSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	    }
	}
	export class DatabaseSpec {
	    type: string;
	    root_pass?: string;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.root_pass = source["root_pass"];
	    }
	}
	export class VersionSpec {
	    version: string;
	    handler: string;
	    extensions: string[];
	    is_default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VersionSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.handler = source["handler"];
	        this.extensions = source["extensions"];
	        this.is_default = source["is_default"];
	    }
	}
	export class PHPSpec {
	    versions: VersionSpec[];
	
	    static createFrom(source: any = {}) {
	        return new PHPSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.versions = this.convertValues(source["versions"], VersionSpec);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WebserverSpec {
	    type: string;
	    enable_http2: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WebserverSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.enable_http2 = source["enable_http2"];
	    }
	}
	export class Blueprint {
	    profile: string;
	    webserver: WebserverSpec;
	    php: PHPSpec;
	    database: DatabaseSpec;
	    cache: CacheSpec;
	    security: SecuritySpec;
	    ssl: SSLSpec;
	    tools: ToolsSpec;
	    containers: ContainersSpec;
	
	    static createFrom(source: any = {}) {
	        return new Blueprint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.webserver = this.convertValues(source["webserver"], WebserverSpec);
	        this.php = this.convertValues(source["php"], PHPSpec);
	        this.database = this.convertValues(source["database"], DatabaseSpec);
	        this.cache = this.convertValues(source["cache"], CacheSpec);
	        this.security = this.convertValues(source["security"], SecuritySpec);
	        this.ssl = this.convertValues(source["ssl"], SSLSpec);
	        this.tools = this.convertValues(source["tools"], ToolsSpec);
	        this.containers = this.convertValues(source["containers"], ContainersSpec);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class PlanItem {
	    category: string;
	    action: string;
	    target: string;
	    description: string;
	    skip: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlanItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category = source["category"];
	        this.action = source["action"];
	        this.target = source["target"];
	        this.description = source["description"];
	        this.skip = source["skip"];
	    }
	}
	export class ProvisioningPlan {
	    server_id: string;
	    profile: string;
	    os_distro: string;
	    os_family: string;
	    os_version: string;
	    arch: string;
	    items: PlanItem[];
	    packages_to_install: string[];
	    repositories_needed: string[];
	    services_to_start: string[];
	    ports_to_open: number[];
	    ssh_port_preserved: number;
	    potential_conflicts: string[];
	    warnings: string[];
	    estimated_time_min: number;
	    is_compatible: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProvisioningPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.profile = source["profile"];
	        this.os_distro = source["os_distro"];
	        this.os_family = source["os_family"];
	        this.os_version = source["os_version"];
	        this.arch = source["arch"];
	        this.items = this.convertValues(source["items"], PlanItem);
	        this.packages_to_install = source["packages_to_install"];
	        this.repositories_needed = source["repositories_needed"];
	        this.services_to_start = source["services_to_start"];
	        this.ports_to_open = source["ports_to_open"];
	        this.ssh_port_preserved = source["ssh_port_preserved"];
	        this.potential_conflicts = source["potential_conflicts"];
	        this.warnings = source["warnings"];
	        this.estimated_time_min = source["estimated_time_min"];
	        this.is_compatible = source["is_compatible"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	

}

export namespace packages {
	
	export class PackageChange {
	    name: string;
	    action: string;
	    old_version?: string;
	    new_version?: string;
	    arch?: string;
	
	    static createFrom(source: any = {}) {
	        return new PackageChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.action = source["action"];
	        this.old_version = source["old_version"];
	        this.new_version = source["new_version"];
	        this.arch = source["arch"];
	    }
	}
	export class AptSimulationResult {
	    success: boolean;
	    command: string;
	    changes: PackageChange[];
	    upgraded_count: number;
	    installed_count: number;
	    removed_count: number;
	    held_count: number;
	    raw_output: string;
	    error_message?: string;
	
	    static createFrom(source: any = {}) {
	        return new AptSimulationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.command = source["command"];
	        this.changes = this.convertValues(source["changes"], PackageChange);
	        this.upgraded_count = source["upgraded_count"];
	        this.installed_count = source["installed_count"];
	        this.removed_count = source["removed_count"];
	        this.held_count = source["held_count"];
	        this.raw_output = source["raw_output"];
	        this.error_message = source["error_message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class PackageItem {
	    name: string;
	    version: string;
	    architecture: string;
	    status: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new PackageItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.architecture = source["architecture"];
	        this.status = source["status"];
	        this.description = source["description"];
	    }
	}
	export class PackageTaskHistoryItem {
	    id: string;
	    server_id: string;
	    operation_type: string;
	    status: string;
	    packages: string[];
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    completed_at?: any;
	    error_message?: string;
	
	    static createFrom(source: any = {}) {
	        return new PackageTaskHistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server_id = source["server_id"];
	        this.operation_type = source["operation_type"];
	        this.status = source["status"];
	        this.packages = source["packages"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.completed_at = this.convertValues(source["completed_at"], null);
	        this.error_message = source["error_message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace php {
	
	export class CreatePoolInput {
	    server_id: string;
	    version: string;
	    pool_name: string;
	    is_enabled: boolean;
	    user: string;
	    group: string;
	    listen_socket: string;
	    listen_owner?: string;
	    listen_group?: string;
	    listen_mode?: string;
	    pm_mode: string;
	    max_children: number;
	    start_servers: number;
	    min_spare_servers: number;
	    max_spare_servers: number;
	    pm_max_requests?: number;
	    process_idle_timeout?: string;
	    memory_limit?: string;
	    max_execution_time?: number;
	    upload_max_filesize?: string;
	    post_max_size?: string;
	    slowlog: boolean;
	    slowlog_timeout?: string;
	    access_log?: string;
	    php_values?: Record<string, string>;
	    php_admin_values?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new CreatePoolInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.version = source["version"];
	        this.pool_name = source["pool_name"];
	        this.is_enabled = source["is_enabled"];
	        this.user = source["user"];
	        this.group = source["group"];
	        this.listen_socket = source["listen_socket"];
	        this.listen_owner = source["listen_owner"];
	        this.listen_group = source["listen_group"];
	        this.listen_mode = source["listen_mode"];
	        this.pm_mode = source["pm_mode"];
	        this.max_children = source["max_children"];
	        this.start_servers = source["start_servers"];
	        this.min_spare_servers = source["min_spare_servers"];
	        this.max_spare_servers = source["max_spare_servers"];
	        this.pm_max_requests = source["pm_max_requests"];
	        this.process_idle_timeout = source["process_idle_timeout"];
	        this.memory_limit = source["memory_limit"];
	        this.max_execution_time = source["max_execution_time"];
	        this.upload_max_filesize = source["upload_max_filesize"];
	        this.post_max_size = source["post_max_size"];
	        this.slowlog = source["slowlog"];
	        this.slowlog_timeout = source["slowlog_timeout"];
	        this.access_log = source["access_log"];
	        this.php_values = source["php_values"];
	        this.php_admin_values = source["php_admin_values"];
	    }
	}
	export class FPMServiceOverview {
	    version: string;
	    service_name: string;
	    status: string;
	    main_pid: number;
	    active_pools: number;
	    total_workers: number;
	    sockets: string[];
	    config_file: string;
	    config_valid: boolean;
	    config_output: string;
	
	    static createFrom(source: any = {}) {
	        return new FPMServiceOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.service_name = source["service_name"];
	        this.status = source["status"];
	        this.main_pid = source["main_pid"];
	        this.active_pools = source["active_pools"];
	        this.total_workers = source["total_workers"];
	        this.sockets = source["sockets"];
	        this.config_file = source["config_file"];
	        this.config_valid = source["config_valid"];
	        this.config_output = source["config_output"];
	    }
	}
	export class FpmPoolConfig {
	    pool_name: string;
	    is_enabled: boolean;
	    user: string;
	    group: string;
	    listen: string;
	    listen_owner?: string;
	    listen_group?: string;
	    listen_mode?: string;
	    pm_mode: string;
	    max_children: number;
	    start_servers: number;
	    min_spare_servers: number;
	    max_spare_servers: number;
	    pm_max_requests?: number;
	    process_idle_timeout?: string;
	    slowlog: boolean;
	    slowlog_timeout?: string;
	    access_log?: string;
	    memory_limit?: string;
	    max_execution_time?: number;
	    upload_max_filesize?: string;
	    post_max_size?: string;
	    php_values?: Record<string, string>;
	    php_admin_values?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new FpmPoolConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pool_name = source["pool_name"];
	        this.is_enabled = source["is_enabled"];
	        this.user = source["user"];
	        this.group = source["group"];
	        this.listen = source["listen"];
	        this.listen_owner = source["listen_owner"];
	        this.listen_group = source["listen_group"];
	        this.listen_mode = source["listen_mode"];
	        this.pm_mode = source["pm_mode"];
	        this.max_children = source["max_children"];
	        this.start_servers = source["start_servers"];
	        this.min_spare_servers = source["min_spare_servers"];
	        this.max_spare_servers = source["max_spare_servers"];
	        this.pm_max_requests = source["pm_max_requests"];
	        this.process_idle_timeout = source["process_idle_timeout"];
	        this.slowlog = source["slowlog"];
	        this.slowlog_timeout = source["slowlog_timeout"];
	        this.access_log = source["access_log"];
	        this.memory_limit = source["memory_limit"];
	        this.max_execution_time = source["max_execution_time"];
	        this.upload_max_filesize = source["upload_max_filesize"];
	        this.post_max_size = source["post_max_size"];
	        this.php_values = source["php_values"];
	        this.php_admin_values = source["php_admin_values"];
	    }
	}
	export class InstallPHPExtensionsPlan {
	    target_version: string;
	    os_family: string;
	    os_distro: string;
	    os_compatible: boolean;
	    extensions: string[];
	    packages_to_install: string[];
	    fpm_service_to_reload: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallPHPExtensionsPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target_version = source["target_version"];
	        this.os_family = source["os_family"];
	        this.os_distro = source["os_distro"];
	        this.os_compatible = source["os_compatible"];
	        this.extensions = source["extensions"];
	        this.packages_to_install = source["packages_to_install"];
	        this.fpm_service_to_reload = source["fpm_service_to_reload"];
	        this.message = source["message"];
	    }
	}
	export class InstallPHPPlan {
	    target_version: string;
	    os_family: string;
	    os_distro: string;
	    os_compatible: boolean;
	    repositories_needed: string[];
	    packages_to_install: string[];
	    fpm_service_to_start: string;
	    estimated_download_mb: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallPHPPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target_version = source["target_version"];
	        this.os_family = source["os_family"];
	        this.os_distro = source["os_distro"];
	        this.os_compatible = source["os_compatible"];
	        this.repositories_needed = source["repositories_needed"];
	        this.packages_to_install = source["packages_to_install"];
	        this.fpm_service_to_start = source["fpm_service_to_start"];
	        this.estimated_download_mb = source["estimated_download_mb"];
	        this.message = source["message"];
	    }
	}
	export class PHPAppRequirement {
	    app_type: string;
	    is_compatible: boolean;
	    required_extensions: string[];
	    missing_extensions: string[];
	    installed_extensions: string[];
	    recommendations: string[];
	
	    static createFrom(source: any = {}) {
	        return new PHPAppRequirement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.app_type = source["app_type"];
	        this.is_compatible = source["is_compatible"];
	        this.required_extensions = source["required_extensions"];
	        this.missing_extensions = source["missing_extensions"];
	        this.installed_extensions = source["installed_extensions"];
	        this.recommendations = source["recommendations"];
	    }
	}
	export class PHPExtensionInfo {
	    name: string;
	    version?: string;
	    is_installed: boolean;
	    is_enabled: boolean;
	    is_core: boolean;
	    category?: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new PHPExtensionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.is_installed = source["is_installed"];
	        this.is_enabled = source["is_enabled"];
	        this.is_core = source["is_core"];
	        this.category = source["category"];
	        this.description = source["description"];
	    }
	}
	export class PHPIniBackupItem {
	    filename: string;
	    path: string;
	    timestamp: number;
	    created_at: string;
	    size_bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new PHPIniBackupItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.path = source["path"];
	        this.timestamp = source["timestamp"];
	        this.created_at = source["created_at"];
	        this.size_bytes = source["size_bytes"];
	    }
	}
	export class PHPSettings {
	    upload_max_filesize: string;
	    post_max_size: string;
	    memory_limit: string;
	    max_execution_time: number;
	    max_input_time: number;
	    max_input_vars: number;
	    date_timezone: string;
	    error_reporting: string;
	    display_errors: boolean;
	    log_errors: boolean;
	    opcache_enable: boolean;
	    opcache_memory_consumption: number;
	    opcache_max_accelerated_files: number;
	    opcache_revalidate_freq: number;
	    short_open_tag: boolean;
	    expose_php: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PHPSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.upload_max_filesize = source["upload_max_filesize"];
	        this.post_max_size = source["post_max_size"];
	        this.memory_limit = source["memory_limit"];
	        this.max_execution_time = source["max_execution_time"];
	        this.max_input_time = source["max_input_time"];
	        this.max_input_vars = source["max_input_vars"];
	        this.date_timezone = source["date_timezone"];
	        this.error_reporting = source["error_reporting"];
	        this.display_errors = source["display_errors"];
	        this.log_errors = source["log_errors"];
	        this.opcache_enable = source["opcache_enable"];
	        this.opcache_memory_consumption = source["opcache_memory_consumption"];
	        this.opcache_max_accelerated_files = source["opcache_max_accelerated_files"];
	        this.opcache_revalidate_freq = source["opcache_revalidate_freq"];
	        this.short_open_tag = source["short_open_tag"];
	        this.expose_php = source["expose_php"];
	    }
	}
	export class PHPVersionInfo {
	    version: string;
	    patch_version: string;
	    update_available: boolean;
	    candidate_version: string;
	    is_installed: boolean;
	    is_available: boolean;
	    unavailable_reason: string;
	    is_default_cli: boolean;
	    default_cli_path?: string;
	    fpm_status: string;
	    fpm_service: string;
	    socket_path: string;
	    ini_path: string;
	    available_handlers: string[];
	    extensions: PHPExtensionInfo[];
	
	    static createFrom(source: any = {}) {
	        return new PHPVersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.patch_version = source["patch_version"];
	        this.update_available = source["update_available"];
	        this.candidate_version = source["candidate_version"];
	        this.is_installed = source["is_installed"];
	        this.is_available = source["is_available"];
	        this.unavailable_reason = source["unavailable_reason"];
	        this.is_default_cli = source["is_default_cli"];
	        this.default_cli_path = source["default_cli_path"];
	        this.fpm_status = source["fpm_status"];
	        this.fpm_service = source["fpm_service"];
	        this.socket_path = source["socket_path"];
	        this.ini_path = source["ini_path"];
	        this.available_handlers = source["available_handlers"];
	        this.extensions = this.convertValues(source["extensions"], PHPExtensionInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace profiles {
	
	export class ProfileInfo {
	    id: string;
	    name: string;
	    description: string;
	    icon: string;
	    recommended: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.icon = source["icon"];
	        this.recommended = source["recommended"];
	    }
	}

}

export namespace router {
	
	export class RouteDecision {
	    provider_type: string;
	    model: string;
	    reason: string;
	    estimated_cost: number;
	    is_fallback: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RouteDecision(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider_type = source["provider_type"];
	        this.model = source["model"];
	        this.reason = source["reason"];
	        this.estimated_cost = source["estimated_cost"];
	        this.is_fallback = source["is_fallback"];
	    }
	}

}

export namespace security {
	
	export class AddFirewallRuleRequest {
	    server_id: string;
	    port: string;
	    protocol: string;
	    action: string;
	    from_ip: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new AddFirewallRuleRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.action = source["action"];
	        this.from_ip = source["from_ip"];
	        this.comment = source["comment"];
	    }
	}
	export class BanIPRequest {
	    server_id: string;
	    jail: string;
	    ip: string;
	
	    static createFrom(source: any = {}) {
	        return new BanIPRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.jail = source["jail"];
	        this.ip = source["ip"];
	    }
	}
	export class DeleteFail2banJailRequest {
	    server_id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new DeleteFail2banJailRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.name = source["name"];
	    }
	}
	export class Fail2banJail {
	    name: string;
	    enabled: boolean;
	    port: string;
	    filter: string;
	    log_path: string;
	    max_retry: number;
	    find_time: string;
	    ban_time: string;
	    action?: string;
	    ignoreip?: string;
	    currently_failed: number;
	    total_failed: number;
	    currently_banned: number;
	    total_banned: number;
	    banned_ip_list: string[];
	    config_file?: string;
	    is_managed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Fail2banJail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.port = source["port"];
	        this.filter = source["filter"];
	        this.log_path = source["log_path"];
	        this.max_retry = source["max_retry"];
	        this.find_time = source["find_time"];
	        this.ban_time = source["ban_time"];
	        this.action = source["action"];
	        this.ignoreip = source["ignoreip"];
	        this.currently_failed = source["currently_failed"];
	        this.total_failed = source["total_failed"];
	        this.currently_banned = source["currently_banned"];
	        this.total_banned = source["total_banned"];
	        this.banned_ip_list = source["banned_ip_list"];
	        this.config_file = source["config_file"];
	        this.is_managed = source["is_managed"];
	    }
	}
	export class Fail2banStatus {
	    is_installed: boolean;
	    is_active: boolean;
	    version: string;
	    jails: Fail2banJail[];
	    available_filters?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Fail2banStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_installed = source["is_installed"];
	        this.is_active = source["is_active"];
	        this.version = source["version"];
	        this.jails = this.convertValues(source["jails"], Fail2banJail);
	        this.available_filters = source["available_filters"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FileSecurityExclusion {
	    id: string;
	    server_id: string;
	    pattern: string;
	    reason: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new FileSecurityExclusion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server_id = source["server_id"];
	        this.pattern = source["pattern"];
	        this.reason = source["reason"];
	        this.created_at = source["created_at"];
	    }
	}
	export class FileSecuritySnapshot {
	    id: string;
	    server_id: string;
	    site_id: string;
	    name: string;
	    base_path: string;
	    files_count: number;
	    total_size_bytes: number;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new FileSecuritySnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.server_id = source["server_id"];
	        this.site_id = source["site_id"];
	        this.name = source["name"];
	        this.base_path = source["base_path"];
	        this.files_count = source["files_count"];
	        this.total_size_bytes = source["total_size_bytes"];
	        this.created_at = source["created_at"];
	    }
	}
	export class FirewallInstallPlan {
	    firewall_type: string;
	    os_family: string;
	    os_distro: string;
	    os_compatible: boolean;
	    packages_to_install: string[];
	    service_to_start: string;
	    already_installed: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new FirewallInstallPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.firewall_type = source["firewall_type"];
	        this.os_family = source["os_family"];
	        this.os_distro = source["os_distro"];
	        this.os_compatible = source["os_compatible"];
	        this.packages_to_install = source["packages_to_install"];
	        this.service_to_start = source["service_to_start"];
	        this.already_installed = source["already_installed"];
	        this.message = source["message"];
	    }
	}
	export class FirewallRule {
	    number: number;
	    port: string;
	    protocol: string;
	    action: string;
	    from_ip: string;
	    direction: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new FirewallRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.action = source["action"];
	        this.from_ip = source["from_ip"];
	        this.direction = source["direction"];
	        this.comment = source["comment"];
	    }
	}
	export class FirewallStatus {
	    type: string;
	    is_active: boolean;
	    default_incoming: string;
	    default_outgoing: string;
	    rules: FirewallRule[];
	
	    static createFrom(source: any = {}) {
	        return new FirewallStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.is_active = source["is_active"];
	        this.default_incoming = source["default_incoming"];
	        this.default_outgoing = source["default_outgoing"];
	        this.rules = this.convertValues(source["rules"], FirewallRule);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ListeningPort {
	    protocol: string;
	    local_address: string;
	    port: number;
	    process: string;
	    pid: number;
	    is_public: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ListeningPort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.local_address = source["local_address"];
	        this.port = source["port"];
	        this.process = source["process"];
	        this.pid = source["pid"];
	        this.is_public = source["is_public"];
	    }
	}
	export class RealtimeSystemMetrics {
	    server_id: string;
	    cpu_percent: number;
	    load_1: number;
	    load_5: number;
	    load_15: number;
	    memory_total_mb: number;
	    memory_used_mb: number;
	    memory_free_mb: number;
	    memory_used_percent: number;
	    swap_total_mb: number;
	    swap_used_mb: number;
	    disk_total_gb: number;
	    disk_used_gb: number;
	    disk_free_gb: number;
	    disk_used_percent: number;
	    uptime_seconds: number;
	    timestamp: string;
	
	    static createFrom(source: any = {}) {
	        return new RealtimeSystemMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.cpu_percent = source["cpu_percent"];
	        this.load_1 = source["load_1"];
	        this.load_5 = source["load_5"];
	        this.load_15 = source["load_15"];
	        this.memory_total_mb = source["memory_total_mb"];
	        this.memory_used_mb = source["memory_used_mb"];
	        this.memory_free_mb = source["memory_free_mb"];
	        this.memory_used_percent = source["memory_used_percent"];
	        this.swap_total_mb = source["swap_total_mb"];
	        this.swap_used_mb = source["swap_used_mb"];
	        this.disk_total_gb = source["disk_total_gb"];
	        this.disk_used_gb = source["disk_used_gb"];
	        this.disk_free_gb = source["disk_free_gb"];
	        this.disk_used_percent = source["disk_used_percent"];
	        this.uptime_seconds = source["uptime_seconds"];
	        this.timestamp = source["timestamp"];
	    }
	}
	export class SaveFail2banJailRequest {
	    server_id: string;
	    name: string;
	    enabled: boolean;
	    port: string;
	    filter: string;
	    log_path: string;
	    max_retry: number;
	    find_time: string;
	    ban_time: string;
	    action?: string;
	    ignoreip?: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveFail2banJailRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.port = source["port"];
	        this.filter = source["filter"];
	        this.log_path = source["log_path"];
	        this.max_retry = source["max_retry"];
	        this.find_time = source["find_time"];
	        this.ban_time = source["ban_time"];
	        this.action = source["action"];
	        this.ignoreip = source["ignoreip"];
	    }
	}
	export class ScanFilesRequest {
	    server_id: string;
	    base_path: string;
	    time_range: string;
	    file_types: string;
	    detect_malware: boolean;
	    max_results: number;
	
	    static createFrom(source: any = {}) {
	        return new ScanFilesRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.base_path = source["base_path"];
	        this.time_range = source["time_range"];
	        this.file_types = source["file_types"];
	        this.detect_malware = source["detect_malware"];
	        this.max_results = source["max_results"];
	    }
	}
	export class ScannedFile {
	    path: string;
	    size: number;
	    formatted_size: string;
	    mod_time: string;
	    permissions: string;
	    owner: string;
	    threat_level: string;
	    reasons: string[];
	    snippet: string;
	    is_excluded: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScannedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.size = source["size"];
	        this.formatted_size = source["formatted_size"];
	        this.mod_time = source["mod_time"];
	        this.permissions = source["permissions"];
	        this.owner = source["owner"];
	        this.threat_level = source["threat_level"];
	        this.reasons = source["reasons"];
	        this.snippet = source["snippet"];
	        this.is_excluded = source["is_excluded"];
	    }
	}
	export class ScanFilesResult {
	    files: ScannedFile[];
	    scanned_count: number;
	    threat_count: number;
	    duration_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new ScanFilesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], ScannedFile);
	        this.scanned_count = source["scanned_count"];
	        this.threat_count = source["threat_count"];
	        this.duration_ms = source["duration_ms"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SecurityCheckItem {
	    key: string;
	    title: string;
	    category: string;
	    status: string;
	    details: string;
	    recommendation: string;
	    can_auto_fix: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SecurityCheckItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.title = source["title"];
	        this.category = source["category"];
	        this.status = source["status"];
	        this.details = source["details"];
	        this.recommendation = source["recommendation"];
	        this.can_auto_fix = source["can_auto_fix"];
	    }
	}
	export class SecurityAuditReport {
	    server_id: string;
	    score: number;
	    ssh_rule_root_login: string;
	    ssh_rule_password_auth: string;
	    ssh_port: number;
	    pending_updates_count: number;
	    security_updates_count: number;
	    listening_ports: ListeningPort[];
	    checks: SecurityCheckItem[];
	    last_audited_at: string;
	
	    static createFrom(source: any = {}) {
	        return new SecurityAuditReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.score = source["score"];
	        this.ssh_rule_root_login = source["ssh_rule_root_login"];
	        this.ssh_rule_password_auth = source["ssh_rule_password_auth"];
	        this.ssh_port = source["ssh_port"];
	        this.pending_updates_count = source["pending_updates_count"];
	        this.security_updates_count = source["security_updates_count"];
	        this.listening_ports = this.convertValues(source["listening_ports"], ListeningPort);
	        this.checks = this.convertValues(source["checks"], SecurityCheckItem);
	        this.last_audited_at = source["last_audited_at"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SiteEndpointCheck {
	    url: string;
	    status_code: number;
	    response_time_ms: number;
	    is_up: boolean;
	    ssl_days_remaining: number;
	    error_message: string;
	    checked_at: string;
	
	    static createFrom(source: any = {}) {
	        return new SiteEndpointCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.status_code = source["status_code"];
	        this.response_time_ms = source["response_time_ms"];
	        this.is_up = source["is_up"];
	        this.ssl_days_remaining = source["ssl_days_remaining"];
	        this.error_message = source["error_message"];
	        this.checked_at = source["checked_at"];
	    }
	}
	export class SnapshotDiffResult {
	    snapshot_id: string;
	    snapshot_name: string;
	    base_path: string;
	    added_files: ScannedFile[];
	    modified_files: ScannedFile[];
	    deleted_files: string[];
	    total_added: number;
	    total_modified: number;
	    total_deleted: number;
	    duration_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotDiffResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snapshot_id = source["snapshot_id"];
	        this.snapshot_name = source["snapshot_name"];
	        this.base_path = source["base_path"];
	        this.added_files = this.convertValues(source["added_files"], ScannedFile);
	        this.modified_files = this.convertValues(source["modified_files"], ScannedFile);
	        this.deleted_files = source["deleted_files"];
	        this.total_added = source["total_added"];
	        this.total_modified = source["total_modified"];
	        this.total_deleted = source["total_deleted"];
	        this.duration_ms = source["duration_ms"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ToggleFail2banJailRequest {
	    server_id: string;
	    name: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ToggleFail2banJailRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	    }
	}
	export class UnbanIPRequest {
	    server_id: string;
	    jail: string;
	    ip: string;
	
	    static createFrom(source: any = {}) {
	        return new UnbanIPRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.jail = source["jail"];
	        this.ip = source["ip"];
	    }
	}
	export class UpdateFirewallRuleRequest {
	    server_id: string;
	    rule_num: number;
	    old_port: string;
	    old_proto: string;
	    port: string;
	    protocol: string;
	    action: string;
	    from_ip: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateFirewallRuleRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.rule_num = source["rule_num"];
	        this.old_port = source["old_port"];
	        this.old_proto = source["old_proto"];
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.action = source["action"];
	        this.from_ip = source["from_ip"];
	        this.comment = source["comment"];
	    }
	}

}

export namespace server {
	
	export class DashboardStats {
	    total_servers: number;
	    online_servers: number;
	    offline_servers: number;
	    unknown_servers: number;
	    total_cores: number;
	    total_ram_bytes: number;
	    total_disk_bytes: number;
	    total_disk_used: number;
	    total_sites: number;
	    active_sites: number;
	    total_ssl_certs: number;
	    expiring_ssl_certs: number;
	    total_backups: number;
	    total_tasks: number;
	    running_tasks: number;
	    failed_tasks: number;
	    recent_tasks: db.TaskRecord[];
	    server_diagnostics: db.DiagnosticsRecord[];
	
	    static createFrom(source: any = {}) {
	        return new DashboardStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_servers = source["total_servers"];
	        this.online_servers = source["online_servers"];
	        this.offline_servers = source["offline_servers"];
	        this.unknown_servers = source["unknown_servers"];
	        this.total_cores = source["total_cores"];
	        this.total_ram_bytes = source["total_ram_bytes"];
	        this.total_disk_bytes = source["total_disk_bytes"];
	        this.total_disk_used = source["total_disk_used"];
	        this.total_sites = source["total_sites"];
	        this.active_sites = source["active_sites"];
	        this.total_ssl_certs = source["total_ssl_certs"];
	        this.expiring_ssl_certs = source["expiring_ssl_certs"];
	        this.total_backups = source["total_backups"];
	        this.total_tasks = source["total_tasks"];
	        this.running_tasks = source["running_tasks"];
	        this.failed_tasks = source["failed_tasks"];
	        this.recent_tasks = this.convertValues(source["recent_tasks"], db.TaskRecord);
	        this.server_diagnostics = this.convertValues(source["server_diagnostics"], db.DiagnosticsRecord);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ServerInput {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    username: string;
	    auth_type: string;
	    secret: string;
	    passphrase: string;
	    sudo_password: string;
	    ssh_host_key: string;
	    group_name: string;
	    tags: string[];
	    is_favorite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ServerInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.auth_type = source["auth_type"];
	        this.secret = source["secret"];
	        this.passphrase = source["passphrase"];
	        this.sudo_password = source["sudo_password"];
	        this.ssh_host_key = source["ssh_host_key"];
	        this.group_name = source["group_name"];
	        this.tags = source["tags"];
	        this.is_favorite = source["is_favorite"];
	    }
	}

}

export namespace services {
	
	export class DNSCheckResult {
	    domain: string;
	    record_type: string;
	    records: string[];
	    success: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DNSCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.domain = source["domain"];
	        this.record_type = source["record_type"];
	        this.records = source["records"];
	        this.success = source["success"];
	        this.error = source["error"];
	    }
	}
	export class ListeningPortItem {
	    proto: string;
	    recv_q: number;
	    send_q: number;
	    local_addr: string;
	    local_port: number;
	    foreign_addr: string;
	    foreign_port: number;
	    process: string;
	    pid: number;
	
	    static createFrom(source: any = {}) {
	        return new ListeningPortItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proto = source["proto"];
	        this.recv_q = source["recv_q"];
	        this.send_q = source["send_q"];
	        this.local_addr = source["local_addr"];
	        this.local_port = source["local_port"];
	        this.foreign_addr = source["foreign_addr"];
	        this.foreign_port = source["foreign_port"];
	        this.process = source["process"];
	        this.pid = source["pid"];
	    }
	}
	export class NetworkOverview {
	    interfaces: string;
	    routes: string;
	    dns_resolv: string;
	    notice: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.interfaces = source["interfaces"];
	        this.routes = source["routes"];
	        this.dns_resolv = source["dns_resolv"];
	        this.notice = source["notice"];
	    }
	}
	export class PortCheckResult {
	    host: string;
	    port: number;
	    open: boolean;
	    latency_ms: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PortCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.open = source["open"];
	        this.latency_ms = source["latency_ms"];
	        this.error = source["error"];
	    }
	}
	export class ProcessItem {
	    user: string;
	    pid: number;
	    cpu: number;
	    mem: number;
	    vsz: number;
	    rss: number;
	    stat: string;
	    start: string;
	    time: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new ProcessItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user = source["user"];
	        this.pid = source["pid"];
	        this.cpu = source["cpu"];
	        this.mem = source["mem"];
	        this.vsz = source["vsz"];
	        this.rss = source["rss"];
	        this.stat = source["stat"];
	        this.start = source["start"];
	        this.time = source["time"];
	        this.command = source["command"];
	    }
	}
	export class SystemUnit {
	    unit: string;
	    load: string;
	    active: string;
	    sub: string;
	    description: string;
	    unit_type: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemUnit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.unit = source["unit"];
	        this.load = source["load"];
	        this.active = source["active"];
	        this.sub = source["sub"];
	        this.description = source["description"];
	        this.unit_type = source["unit_type"];
	    }
	}

}

export namespace site {
	
	export class CreateSitePlan {
	    domain: string;
	    server_host: string;
	    document_root: string;
	    config_file: string;
	    access_log_file: string;
	    error_log_file: string;
	    php_version: string;
	    fpm_socket: string;
	    webserver_to_reload: string;
	    dns_matches: boolean;
	    preflight_warnings: string[];
	    owner_user: string;
	    packages_to_install: string[];
	    existing_site_conflict: boolean;
	    conflict_message: string;
	    database_planned: boolean;
	    database_name: string;
	    database_user: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateSitePlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.domain = source["domain"];
	        this.server_host = source["server_host"];
	        this.document_root = source["document_root"];
	        this.config_file = source["config_file"];
	        this.access_log_file = source["access_log_file"];
	        this.error_log_file = source["error_log_file"];
	        this.php_version = source["php_version"];
	        this.fpm_socket = source["fpm_socket"];
	        this.webserver_to_reload = source["webserver_to_reload"];
	        this.dns_matches = source["dns_matches"];
	        this.preflight_warnings = source["preflight_warnings"];
	        this.owner_user = source["owner_user"];
	        this.packages_to_install = source["packages_to_install"];
	        this.existing_site_conflict = source["existing_site_conflict"];
	        this.conflict_message = source["conflict_message"];
	        this.database_planned = source["database_planned"];
	        this.database_name = source["database_name"];
	        this.database_user = source["database_user"];
	    }
	}
	export class SiteConfigFileResult {
	    site_id: string;
	    domain: string;
	    file_path: string;
	    content: string;
	    webserver_type: string;
	
	    static createFrom(source: any = {}) {
	        return new SiteConfigFileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.site_id = source["site_id"];
	        this.domain = source["domain"];
	        this.file_path = source["file_path"];
	        this.content = source["content"];
	        this.webserver_type = source["webserver_type"];
	    }
	}
	export class SiteDNSCheckResult {
	    domain: string;
	    resolved_ips: string[];
	    server_ip: string;
	    match: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new SiteDNSCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.domain = source["domain"];
	        this.resolved_ips = source["resolved_ips"];
	        this.server_ip = source["server_ip"];
	        this.match = source["match"];
	        this.message = source["message"];
	    }
	}
	export class SiteDeletePlan {
	    site_id: string;
	    domain: string;
	    origin: string;
	    web_server: string;
	    document_root: string;
	    delete_files: boolean;
	    is_discovered: boolean;
	    warning_message: string;
	    description: string;
	    files_warning: string;
	
	    static createFrom(source: any = {}) {
	        return new SiteDeletePlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.site_id = source["site_id"];
	        this.domain = source["domain"];
	        this.origin = source["origin"];
	        this.web_server = source["web_server"];
	        this.document_root = source["document_root"];
	        this.delete_files = source["delete_files"];
	        this.is_discovered = source["is_discovered"];
	        this.warning_message = source["warning_message"];
	        this.description = source["description"];
	        this.files_warning = source["files_warning"];
	    }
	}
	export class SiteInput {
	    server_id: string;
	    domain: string;
	    aliases: string[];
	    document_root: string;
	    site_type: string;
	    php_version: string;
	    fpm_pool: string;
	    web_server: string;
	    proxy_pass_url: string;
	    ssl_enabled: boolean;
	    create_database: boolean;
	    database_name: string;
	    database_user: string;
	    database_password: string;
	    enable_logs: boolean;
	    disable_logs: boolean;
	    redirect_https: boolean;
	    http2_supported: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SiteInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.domain = source["domain"];
	        this.aliases = source["aliases"];
	        this.document_root = source["document_root"];
	        this.site_type = source["site_type"];
	        this.php_version = source["php_version"];
	        this.fpm_pool = source["fpm_pool"];
	        this.web_server = source["web_server"];
	        this.proxy_pass_url = source["proxy_pass_url"];
	        this.ssl_enabled = source["ssl_enabled"];
	        this.create_database = source["create_database"];
	        this.database_name = source["database_name"];
	        this.database_user = source["database_user"];
	        this.database_password = source["database_password"];
	        this.enable_logs = source["enable_logs"];
	        this.disable_logs = source["disable_logs"];
	        this.redirect_https = source["redirect_https"];
	        this.http2_supported = source["http2_supported"];
	    }
	}
	export class SiteRuntimeInput {
	    site_id: string;
	    php_version: string;
	    handler: string;
	    pool_name: string;
	    custom_listen_socket: string;
	
	    static createFrom(source: any = {}) {
	        return new SiteRuntimeInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.site_id = source["site_id"];
	        this.php_version = source["php_version"];
	        this.handler = source["handler"];
	        this.pool_name = source["pool_name"];
	        this.custom_listen_socket = source["custom_listen_socket"];
	    }
	}
	export class SiteRuntimePlan {
	    site_id: string;
	    domain: string;
	    current_php_version: string;
	    target_php_version: string;
	    handler: string;
	    pool_name: string;
	    target_socket: string;
	    webserver: string;
	    vhost_file: string;
	    is_compatible: boolean;
	    missing_extensions: string[];
	    preflight_warnings: string[];
	    diff_preview: string;
	
	    static createFrom(source: any = {}) {
	        return new SiteRuntimePlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.site_id = source["site_id"];
	        this.domain = source["domain"];
	        this.current_php_version = source["current_php_version"];
	        this.target_php_version = source["target_php_version"];
	        this.handler = source["handler"];
	        this.pool_name = source["pool_name"];
	        this.target_socket = source["target_socket"];
	        this.webserver = source["webserver"];
	        this.vhost_file = source["vhost_file"];
	        this.is_compatible = source["is_compatible"];
	        this.missing_extensions = source["missing_extensions"];
	        this.preflight_warnings = source["preflight_warnings"];
	        this.diff_preview = source["diff_preview"];
	    }
	}

}

export namespace siteaudit {
	
	export class Finding {
	    path: string;
	    rule_id: string;
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new Finding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.rule_id = source["rule_id"];
	        this.summary = source["summary"];
	    }
	}
	export class Snapshot {
	    id: string;
	    site_id: string;
	    server_id: string;
	    document_root: string;
	    is_baseline: boolean;
	    file_count: number;
	    added: string[];
	    removed: string[];
	    changed: string[];
	    findings: Finding[];
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.site_id = source["site_id"];
	        this.server_id = source["server_id"];
	        this.document_root = source["document_root"];
	        this.is_baseline = source["is_baseline"];
	        this.file_count = source["file_count"];
	        this.added = source["added"];
	        this.removed = source["removed"];
	        this.changed = source["changed"];
	        this.findings = this.convertValues(source["findings"], Finding);
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace ssh {
	
	export class HostKeyInspection {
	    status: string;
	    fingerprint: string;
	    key_type: string;
	    raw_entry: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new HostKeyInspection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.fingerprint = source["fingerprint"];
	        this.key_type = source["key_type"];
	        this.raw_entry = source["raw_entry"];
	        this.message = source["message"];
	    }
	}

}

export namespace ssl {
	
	export class CertbotCertificateItem {
	    name: string;
	    key_type: string;
	    domains: string[];
	    expiry_date: string;
	    days_left: number;
	    is_expired: boolean;
	    cert_path: string;
	    key_path: string;
	
	    static createFrom(source: any = {}) {
	        return new CertbotCertificateItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.key_type = source["key_type"];
	        this.domains = source["domains"];
	        this.expiry_date = source["expiry_date"];
	        this.days_left = source["days_left"];
	        this.is_expired = source["is_expired"];
	        this.cert_path = source["cert_path"];
	        this.key_path = source["key_path"];
	    }
	}
	export class CertbotConfig {
	    email: string;
	    key_type: string;
	    rsa_key_size: number;
	    elliptic_curve: string;
	    deploy_hook: string;
	    auto_renew: boolean;
	    agree_tos: boolean;
	    raw_cli_ini: string;
	
	    static createFrom(source: any = {}) {
	        return new CertbotConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.key_type = source["key_type"];
	        this.rsa_key_size = source["rsa_key_size"];
	        this.elliptic_curve = source["elliptic_curve"];
	        this.deploy_hook = source["deploy_hook"];
	        this.auto_renew = source["auto_renew"];
	        this.agree_tos = source["agree_tos"];
	        this.raw_cli_ini = source["raw_cli_ini"];
	    }
	}
	export class CertbotInstallPlan {
	    os_family: string;
	    os_distro: string;
	    os_compatible: boolean;
	    packages_to_install: string[];
	    service_to_start: string;
	    webserver: string;
	    already_installed: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CertbotInstallPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os_family = source["os_family"];
	        this.os_distro = source["os_distro"];
	        this.os_compatible = source["os_compatible"];
	        this.packages_to_install = source["packages_to_install"];
	        this.service_to_start = source["service_to_start"];
	        this.webserver = source["webserver"];
	        this.already_installed = source["already_installed"];
	        this.message = source["message"];
	    }
	}
	export class CertbotRenewResult {
	    success: boolean;
	    output: string;
	
	    static createFrom(source: any = {}) {
	        return new CertbotRenewResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.output = source["output"];
	    }
	}
	export class CertbotStatus {
	    installed: boolean;
	    version: string;
	    path: string;
	    renew_timer_active: boolean;
	    webserver: string;
	
	    static createFrom(source: any = {}) {
	        return new CertbotStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.version = source["version"];
	        this.path = source["path"];
	        this.renew_timer_active = source["renew_timer_active"];
	        this.webserver = source["webserver"];
	    }
	}
	export class IssueSSLInput {
	    server_id: string;
	    site_id: string;
	    primary_domain: string;
	    san_domains: string[];
	    email: string;
	    challenge_type: string;
	    auto_renew: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IssueSSLInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.site_id = source["site_id"];
	        this.primary_domain = source["primary_domain"];
	        this.san_domains = source["san_domains"];
	        this.email = source["email"];
	        this.challenge_type = source["challenge_type"];
	        this.auto_renew = source["auto_renew"];
	    }
	}
	export class SSLMetrics {
	    total_certificates: number;
	    valid_count: number;
	    expiring_soon_count: number;
	    expired_count: number;
	
	    static createFrom(source: any = {}) {
	        return new SSLMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_certificates = source["total_certificates"];
	        this.valid_count = source["valid_count"];
	        this.expiring_soon_count = source["expiring_soon_count"];
	        this.expired_count = source["expired_count"];
	    }
	}

}

export namespace terminal {
	
	export class ResizeTerminalRequest {
	    session_id: string;
	    cols: number;
	    rows: number;
	
	    static createFrom(source: any = {}) {
	        return new ResizeTerminalRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session_id = source["session_id"];
	        this.cols = source["cols"];
	        this.rows = source["rows"];
	    }
	}
	export class StartSessionRequest {
	    server_id: string;
	    cols: number;
	    rows: number;
	
	    static createFrom(source: any = {}) {
	        return new StartSessionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_id = source["server_id"];
	        this.cols = source["cols"];
	        this.rows = source["rows"];
	    }
	}
	export class TerminalSessionInfo {
	    session_id: string;
	    server_id: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new TerminalSessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session_id = source["session_id"];
	        this.server_id = source["server_id"];
	        this.created_at = source["created_at"];
	    }
	}

}

export namespace update {
	
	export class Asset {
	    id: number;
	    name: string;
	    browser_download_url: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new Asset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.browser_download_url = source["browser_download_url"];
	        this.size = source["size"];
	    }
	}
	export class Release {
	    tag_name: string;
	    name: string;
	    body: string;
	    html_url: string;
	    published_at: string;
	    assets: Asset[];
	
	    static createFrom(source: any = {}) {
	        return new Release(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tag_name = source["tag_name"];
	        this.name = source["name"];
	        this.body = source["body"];
	        this.html_url = source["html_url"];
	        this.published_at = source["published_at"];
	        this.assets = this.convertValues(source["assets"], Asset);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CheckResult {
	    current_version: string;
	    latest_version: string;
	    available: boolean;
	    asset_found: boolean;
	    release?: Release;
	
	    static createFrom(source: any = {}) {
	        return new CheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current_version = source["current_version"];
	        this.latest_version = source["latest_version"];
	        this.available = source["available"];
	        this.asset_found = source["asset_found"];
	        this.release = this.convertValues(source["release"], Release);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace users {
	
	export class AuthorizedKeyItem {
	    key_type: string;
	    key_data: string;
	    comment: string;
	    raw: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthorizedKeyItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key_type = source["key_type"];
	        this.key_data = source["key_data"];
	        this.comment = source["comment"];
	        this.raw = source["raw"];
	    }
	}
	export class SystemUserInfo {
	    username: string;
	    uid: number;
	    gid: number;
	    home_dir: string;
	    shell: string;
	    is_system: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SystemUserInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.uid = source["uid"];
	        this.gid = source["gid"];
	        this.home_dir = source["home_dir"];
	        this.shell = source["shell"];
	        this.is_system = source["is_system"];
	    }
	}

}

export namespace webserver {
	
	export class InstallPlan {
	    webserver_type: string;
	    os_family: string;
	    os_distro: string;
	    os_compatible: boolean;
	    packages_to_install: string[];
	    service_to_start: string;
	    port_conflict_warning?: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.webserver_type = source["webserver_type"];
	        this.os_family = source["os_family"];
	        this.os_distro = source["os_distro"];
	        this.os_compatible = source["os_compatible"];
	        this.packages_to_install = source["packages_to_install"];
	        this.service_to_start = source["service_to_start"];
	        this.port_conflict_warning = source["port_conflict_warning"];
	        this.message = source["message"];
	    }
	}
	export class WebServerStatus {
	    name: string;
	    display_name: string;
	    is_installed: boolean;
	    status: string;
	    version: string;
	    config_path: string;
	    test_output: string;
	    test_passed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WebServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.display_name = source["display_name"];
	        this.is_installed = source["is_installed"];
	        this.status = source["status"];
	        this.version = source["version"];
	        this.config_path = source["config_path"];
	        this.test_output = source["test_output"];
	        this.test_passed = source["test_passed"];
	    }
	}

}

export namespace workflows {
	
	export class Issue {
	    id: string;
	    severity: string;
	    component: string;
	    title: string;
	    description: string;
	    recommendation: string;
	    proposed_action?: string;
	    action_params?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Issue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.severity = source["severity"];
	        this.component = source["component"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.recommendation = source["recommendation"];
	        this.proposed_action = source["proposed_action"];
	        this.action_params = source["action_params"];
	    }
	}
	export class DiagnosticReport {
	    workflow_type: string;
	    server_id: string;
	    server_name?: string;
	    target?: string;
	    status: string;
	    summary: string;
	    // Go type: time
	    timestamp: any;
	    issues: Issue[];
	    metrics?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workflow_type = source["workflow_type"];
	        this.server_id = source["server_id"];
	        this.server_name = source["server_name"];
	        this.target = source["target"];
	        this.status = source["status"];
	        this.summary = source["summary"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.issues = this.convertValues(source["issues"], Issue);
	        this.metrics = source["metrics"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

