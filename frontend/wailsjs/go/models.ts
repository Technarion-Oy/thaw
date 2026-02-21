export namespace config {
	
	export class GitConfig {
	    exportDir: string;
	    remoteURL: string;
	    branch: string;
	    authorName: string;
	    authorEmail: string;
	
	    static createFrom(source: any = {}) {
	        return new GitConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exportDir = source["exportDir"];
	        this.remoteURL = source["remoteURL"];
	        this.branch = source["branch"];
	        this.authorName = source["authorName"];
	        this.authorEmail = source["authorEmail"];
	    }
	}

}

export namespace ddl {
	
	export class ExportResult {
	    database: string;
	    files: number;
	    skipped: number;
	    errors?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.files = source["files"];
	        this.skipped = source["skipped"];
	        this.errors = source["errors"];
	    }
	}

}

export namespace filesystem {
	
	export class FileEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.size = source["size"];
	    }
	}

}

export namespace gitrepo {
	
	export class PullParams {
	    dir: string;
	    remoteURL: string;
	    branch: string;
	    token: string;
	
	    static createFrom(source: any = {}) {
	        return new PullParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.remoteURL = source["remoteURL"];
	        this.branch = source["branch"];
	        this.token = source["token"];
	    }
	}
	export class PushParams {
	    dir: string;
	    remoteURL: string;
	    branch: string;
	    token: string;
	    message: string;
	    authorName: string;
	    authorEmail: string;
	    files: string[];
	
	    static createFrom(source: any = {}) {
	        return new PushParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.remoteURL = source["remoteURL"];
	        this.branch = source["branch"];
	        this.token = source["token"];
	        this.message = source["message"];
	        this.authorName = source["authorName"];
	        this.authorEmail = source["authorEmail"];
	        this.files = source["files"];
	    }
	}
	export class RepoStatus {
	    isRepo: boolean;
	    branch: string;
	    modified: string[];
	    added: string[];
	    deleted: string[];
	    hasRemote: boolean;
	    remoteURL: string;
	    ahead: number;
	
	    static createFrom(source: any = {}) {
	        return new RepoStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isRepo = source["isRepo"];
	        this.branch = source["branch"];
	        this.modified = source["modified"];
	        this.added = source["added"];
	        this.deleted = source["deleted"];
	        this.hasRemote = source["hasRemote"];
	        this.remoteURL = source["remoteURL"];
	        this.ahead = source["ahead"];
	    }
	}

}

export namespace sfconfig {
	
	export class Connection {
	    name: string;
	    account: string;
	    user: string;
	    password: string;
	    role: string;
	    warehouse: string;
	    database: string;
	    schema: string;
	    authenticator: string;
	    passcode: string;
	    oktaUrl: string;
	    privateKeyPath: string;
	    privateKeyPassphrase: string;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.account = source["account"];
	        this.user = source["user"];
	        this.password = source["password"];
	        this.role = source["role"];
	        this.warehouse = source["warehouse"];
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.authenticator = source["authenticator"];
	        this.passcode = source["passcode"];
	        this.oktaUrl = source["oktaUrl"];
	        this.privateKeyPath = source["privateKeyPath"];
	        this.privateKeyPassphrase = source["privateKeyPassphrase"];
	    }
	}
	export class Config {
	    defaultConnection: string;
	    connections: Connection[];
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultConnection = source["defaultConnection"];
	        this.connections = this.convertValues(source["connections"], Connection);
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

export namespace snowflake {
	
	export class ConnectParams {
	    account: string;
	    user: string;
	    password: string;
	    role: string;
	    warehouse: string;
	    database: string;
	    schema: string;
	    authenticator: string;
	    passcode: string;
	    oktaUrl: string;
	    privateKeyPath: string;
	    privateKeyPassphrase: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.account = source["account"];
	        this.user = source["user"];
	        this.password = source["password"];
	        this.role = source["role"];
	        this.warehouse = source["warehouse"];
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.authenticator = source["authenticator"];
	        this.passcode = source["passcode"];
	        this.oktaUrl = source["oktaUrl"];
	        this.privateKeyPath = source["privateKeyPath"];
	        this.privateKeyPassphrase = source["privateKeyPassphrase"];
	    }
	}
	export class QueryResult {
	    columns: string[];
	    rows: any[][];
	    rowsAffected: number;
	
	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.rowsAffected = source["rowsAffected"];
	    }
	}
	export class SessionContext {
	    role: string;
	    warehouse: string;
	    database: string;
	    schema: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionContext(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.warehouse = source["warehouse"];
	        this.database = source["database"];
	        this.schema = source["schema"];
	    }
	}
	export class SnowflakeObject {
	    name: string;
	    kind: string;
	    schema: string;
	    arguments: string;
	
	    static createFrom(source: any = {}) {
	        return new SnowflakeObject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.schema = source["schema"];
	        this.arguments = source["arguments"];
	    }
	}

}

