export namespace ai {
	
	export class UIToolCall {
	    name: string;
	    input: string;
	    output: string;
	    isError: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UIToolCall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.input = source["input"];
	        this.output = source["output"];
	        this.isError = source["isError"];
	    }
	}
	export class UIMessage {
	    role: string;
	    text: string;
	    toolCalls?: UIToolCall[];
	
	    static createFrom(source: any = {}) {
	        return new UIMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.text = source["text"];
	        this.toolCalls = this.convertValues(source["toolCalls"], UIToolCall);
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

export namespace config {
	
	export class AIConfig {
	    provider: string;
	    apiKey: string;
	    model: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.apiKey = source["apiKey"];
	        this.model = source["model"];
	        this.enabled = source["enabled"];
	    }
	}
	export class GitConfig {
	    exportDir: string;
	    remoteURL: string;
	    branch: string;
	    authorName: string;
	    authorEmail: string;
	    exportPathTemplate: string;
	
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
	        this.exportPathTemplate = source["exportPathTemplate"];
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
	export class SearchMatch {
	    path: string;
	    lineNumber: number;
	    lineContent: string;
	    matchStart: number;
	    matchEnd: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchMatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.lineNumber = source["lineNumber"];
	        this.lineContent = source["lineContent"];
	        this.matchStart = source["matchStart"];
	        this.matchEnd = source["matchEnd"];
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

export namespace main {
	
	export class AccountExportResult {
	    roles: number;
	    warehouses: number;
	    errors?: string[];
	
	    static createFrom(source: any = {}) {
	        return new AccountExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.roles = source["roles"];
	        this.warehouses = source["warehouses"];
	        this.errors = source["errors"];
	    }
	}
	export class BackupPolicyRow {
	    name: string;
	    createdOn: string;
	    owner: string;
	    schedule: string;
	    expireAfterDays: number;
	    retentionLock: boolean;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupPolicyRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.createdOn = source["createdOn"];
	        this.owner = source["owner"];
	        this.schedule = source["schedule"];
	        this.expireAfterDays = source["expireAfterDays"];
	        this.retentionLock = source["retentionLock"];
	        this.comment = source["comment"];
	    }
	}
	export class BackupRow {
	    id: string;
	    name: string;
	    createdOn: string;
	    status: string;
	    sizeBytes: number;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.createdOn = source["createdOn"];
	        this.status = source["status"];
	        this.sizeBytes = source["sizeBytes"];
	        this.comment = source["comment"];
	    }
	}
	export class BackupSetRow {
	    name: string;
	    backupSetDb: string;
	    backupSetSchema: string;
	    createdOn: string;
	    objectType: string;
	    objectName: string;
	    objectDb: string;
	    objectSchema: string;
	    status: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupSetRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.backupSetDb = source["backupSetDb"];
	        this.backupSetSchema = source["backupSetSchema"];
	        this.createdOn = source["createdOn"];
	        this.objectType = source["objectType"];
	        this.objectName = source["objectName"];
	        this.objectDb = source["objectDb"];
	        this.objectSchema = source["objectSchema"];
	        this.status = source["status"];
	        this.comment = source["comment"];
	    }
	}
	export class ColumnComment {
	    column: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new ColumnComment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.column = source["column"];
	        this.comment = source["comment"];
	    }
	}
	export class PropertyPair {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new PropertyPair(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class QueryHistoryRow {
	    queryId: string;
	    queryText: string;
	    queryType: string;
	    userName: string;
	    warehouseName: string;
	    databaseName: string;
	    schemaName: string;
	    startTime: string;
	    endTime: string;
	    elapsedMs: number;
	    status: string;
	    errorMessage: string;
	    rowsProduced: number;
	    bytesScanned: number;
	
	    static createFrom(source: any = {}) {
	        return new QueryHistoryRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.queryId = source["queryId"];
	        this.queryText = source["queryText"];
	        this.queryType = source["queryType"];
	        this.userName = source["userName"];
	        this.warehouseName = source["warehouseName"];
	        this.databaseName = source["databaseName"];
	        this.schemaName = source["schemaName"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.elapsedMs = source["elapsedMs"];
	        this.status = source["status"];
	        this.errorMessage = source["errorMessage"];
	        this.rowsProduced = source["rowsProduced"];
	        this.bytesScanned = source["bytesScanned"];
	    }
	}
	export class SessionParam {
	    key: string;
	    value: string;
	    type: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionParam(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	        this.type = source["type"];
	        this.description = source["description"];
	    }
	}
	export class SessionVar {
	    key: string;
	    value: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionVar(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	        this.type = source["type"];
	    }
	}
	export class TableSettings {
	    clusterBy: string;
	    enableSchemaEvolution: boolean;
	    dataRetentionDays: number;
	    maxDataExtensionDays: number;
	    changeTracking: boolean;
	    defaultDDLCollation: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new TableSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clusterBy = source["clusterBy"];
	        this.enableSchemaEvolution = source["enableSchemaEvolution"];
	        this.dataRetentionDays = source["dataRetentionDays"];
	        this.maxDataExtensionDays = source["maxDataExtensionDays"];
	        this.changeTracking = source["changeTracking"];
	        this.defaultDDLCollation = source["defaultDDLCollation"];
	        this.comment = source["comment"];
	    }
	}
	export class WarehouseMeteringRow {
	    startTime: string;
	    endTime: string;
	    warehouseName: string;
	    creditsUsed: number;
	    creditsUsedCompute: number;
	    creditsUsedCloudServices: number;
	
	    static createFrom(source: any = {}) {
	        return new WarehouseMeteringRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.warehouseName = source["warehouseName"];
	        this.creditsUsed = source["creditsUsed"];
	        this.creditsUsedCompute = source["creditsUsedCompute"];
	        this.creditsUsedCloudServices = source["creditsUsedCloudServices"];
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
	export class DroppedTable {
	    name: string;
	    droppedOn: string;
	
	    static createFrom(source: any = {}) {
	        return new DroppedTable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.droppedOn = source["droppedOn"];
	    }
	}
	export class ERColumn {
	    name: string;
	    dataType: string;
	    isPK: boolean;
	    nullable: string;
	
	    static createFrom(source: any = {}) {
	        return new ERColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dataType = source["dataType"];
	        this.isPK = source["isPK"];
	        this.nullable = source["nullable"];
	    }
	}
	export class ERForeignKey {
	    fromSchema: string;
	    fromTable: string;
	    fromCol: string;
	    toSchema: string;
	    toTable: string;
	    toCol: string;
	
	    static createFrom(source: any = {}) {
	        return new ERForeignKey(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fromSchema = source["fromSchema"];
	        this.fromTable = source["fromTable"];
	        this.fromCol = source["fromCol"];
	        this.toSchema = source["toSchema"];
	        this.toTable = source["toTable"];
	        this.toCol = source["toCol"];
	    }
	}
	export class ERTable {
	    schema: string;
	    name: string;
	    columns: ERColumn[];
	
	    static createFrom(source: any = {}) {
	        return new ERTable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.columns = this.convertValues(source["columns"], ERColumn);
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
	export class ERDiagramData {
	    database: string;
	    tables: ERTable[];
	    fks: ERForeignKey[];
	
	    static createFrom(source: any = {}) {
	        return new ERDiagramData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.tables = this.convertValues(source["tables"], ERTable);
	        this.fks = this.convertValues(source["fks"], ERForeignKey);
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
	
	
	export class ExportTableParams {
	    database: string;
	    schema: string;
	    table: string;
	    outputDir: string;
	    format: string;
	    compression: string;
	    delimiter: string;
	    header: boolean;
	    nullString: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportTableParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.outputDir = source["outputDir"];
	        this.format = source["format"];
	        this.compression = source["compression"];
	        this.delimiter = source["delimiter"];
	        this.header = source["header"];
	        this.nullString = source["nullString"];
	    }
	}
	export class ExportTableResult {
	    rowsUnloaded: number;
	    files: string[];
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportTableResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rowsUnloaded = source["rowsUnloaded"];
	        this.files = source["files"];
	        this.outputDir = source["outputDir"];
	    }
	}
	export class ProcParam {
	    name: string;
	    dataType: string;
	
	    static createFrom(source: any = {}) {
	        return new ProcParam(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dataType = source["dataType"];
	    }
	}
	export class FunctionInfo {
	    params: ProcParam[];
	    isTableFunction: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FunctionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.params = this.convertValues(source["params"], ProcParam);
	        this.isTableFunction = source["isTableFunction"];
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
	export class ImportTableParams {
	    database: string;
	    schema: string;
	    table: string;
	    filePath: string;
	    format: string;
	    delimiter: string;
	    header: boolean;
	    nullString: string;
	    overwrite: boolean;
	    createTable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ImportTableParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.filePath = source["filePath"];
	        this.format = source["format"];
	        this.delimiter = source["delimiter"];
	        this.header = source["header"];
	        this.nullString = source["nullString"];
	        this.overwrite = source["overwrite"];
	        this.createTable = source["createTable"];
	    }
	}
	export class ImportTableResult {
	    rowsLoaded: number;
	    filesLoaded: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportTableResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rowsLoaded = source["rowsLoaded"];
	        this.filesLoaded = source["filesLoaded"];
	    }
	}
	
	export class QueryResult {
	    columns: string[];
	    rows: any[][];
	    rowsAffected: number;
	    queryID: string;
	
	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.rowsAffected = source["rowsAffected"];
	        this.queryID = source["queryID"];
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
	    rowCount?: number;
	
	    static createFrom(source: any = {}) {
	        return new SnowflakeObject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.schema = source["schema"];
	        this.arguments = source["arguments"];
	        this.rowCount = source["rowCount"];
	    }
	}
	export class SnowflakeUser {
	    name: string;
	    loginName: string;
	    displayName: string;
	    firstName: string;
	    lastName: string;
	    email: string;
	    defaultWarehouse: string;
	    defaultRole: string;
	    defaultNamespace: string;
	    comment: string;
	    disabled: boolean;
	    mustChangePassword: boolean;
	    daysToExpiry: string;
	    owner: string;
	    lastSuccessLogin: string;
	
	    static createFrom(source: any = {}) {
	        return new SnowflakeUser(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.loginName = source["loginName"];
	        this.displayName = source["displayName"];
	        this.firstName = source["firstName"];
	        this.lastName = source["lastName"];
	        this.email = source["email"];
	        this.defaultWarehouse = source["defaultWarehouse"];
	        this.defaultRole = source["defaultRole"];
	        this.defaultNamespace = source["defaultNamespace"];
	        this.comment = source["comment"];
	        this.disabled = source["disabled"];
	        this.mustChangePassword = source["mustChangePassword"];
	        this.daysToExpiry = source["daysToExpiry"];
	        this.owner = source["owner"];
	        this.lastSuccessLogin = source["lastSuccessLogin"];
	    }
	}

}

