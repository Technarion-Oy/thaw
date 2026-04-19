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
	    ollamaPort?: number;
	    ollamaNumCtx?: number;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.apiKey = source["apiKey"];
	        this.model = source["model"];
	        this.enabled = source["enabled"];
	        this.ollamaPort = source["ollamaPort"];
	        this.ollamaNumCtx = source["ollamaNumCtx"];
	    }
	}
	export class EditorPrefs {
	    keywordCase: string;
	    identifierCase: string;
	    functionCase: string;
	    indentStyle: string;
	    indentSize: number;
	    commaPosition: string;
	    operatorPosition: string;
	
	    static createFrom(source: any = {}) {
	        return new EditorPrefs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keywordCase = source["keywordCase"];
	        this.identifierCase = source["identifierCase"];
	        this.functionCase = source["functionCase"];
	        this.indentStyle = source["indentStyle"];
	        this.indentSize = source["indentSize"];
	        this.commaPosition = source["commaPosition"];
	        this.operatorPosition = source["operatorPosition"];
	    }
	}
	export class FeatureFlags {
	    initialized: boolean;
	    exportTableData: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FeatureFlags(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.initialized = source["initialized"];
	        this.exportTableData = source["exportTableData"];
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
	export class NotebookPrefs {
	    syntaxMode: string;
	
	    static createFrom(source: any = {}) {
	        return new NotebookPrefs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.syntaxMode = source["syntaxMode"];
	    }
	}

}

export namespace dbt {
	
	export class CreateRequest {
	    projectName: string;
	    outputDir: string;
	    profileName: string;
	    inlineViewDefs: boolean;
	    databaseVars: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CreateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectName = source["projectName"];
	        this.outputDir = source["outputDir"];
	        this.profileName = source["profileName"];
	        this.inlineViewDefs = source["inlineViewDefs"];
	        this.databaseVars = source["databaseVars"];
	    }
	}
	export class CreateResult {
	    projectDir: string;
	    filesCreated: string[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new CreateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectDir = source["projectDir"];
	        this.filesCreated = source["filesCreated"];
	        this.warnings = source["warnings"];
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

export namespace fnmeta {
	
	export class FunctionMeta {
	    functionName: string;
	    functionSignature: string;
	    description: string;
	    functionType: string;
	
	    static createFrom(source: any = {}) {
	        return new FunctionMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.functionName = source["functionName"];
	        this.functionSignature = source["functionSignature"];
	        this.description = source["description"];
	        this.functionType = source["functionType"];
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
	export class KeyPairResult {
	    privateKeyPath: string;
	    publicKeyPath: string;
	    publicKey: string;
	
	    static createFrom(source: any = {}) {
	        return new KeyPairResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.privateKeyPath = source["privateKeyPath"];
	        this.publicKeyPath = source["publicKeyPath"];
	        this.publicKey = source["publicKey"];
	    }
	}
	export class MigrationObject {
	    filePath: string;
	    database: string;
	    schema: string;
	    objectKind: string;
	    objectName: string;
	    argSig: string;
	    ddl: string;
	    isReplace: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MigrationObject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.objectKind = source["objectKind"];
	        this.objectName = source["objectName"];
	        this.argSig = source["argSig"];
	        this.ddl = source["ddl"];
	        this.isReplace = source["isReplace"];
	    }
	}
	export class MigrationDiffItem {
	    object: MigrationObject;
	    status: string;
	    localDDL: string;
	    remoteDDL: string;
	
	    static createFrom(source: any = {}) {
	        return new MigrationDiffItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.object = this.convertValues(source["object"], MigrationObject);
	        this.status = source["status"];
	        this.localDDL = source["localDDL"];
	        this.remoteDDL = source["remoteDDL"];
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
	export class MigrationExecEvent {
	    done: number;
	    total: number;
	    object: string;
	    status: string;
	    error: string;
	    pass: number;
	
	    static createFrom(source: any = {}) {
	        return new MigrationExecEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.done = source["done"];
	        this.total = source["total"];
	        this.object = source["object"];
	        this.status = source["status"];
	        this.error = source["error"];
	        this.pass = source["pass"];
	    }
	}
	
	export class NotebookSessionContext {
	    role: string;
	    warehouse: string;
	    database: string;
	    schema: string;
	
	    static createFrom(source: any = {}) {
	        return new NotebookSessionContext(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.warehouse = source["warehouse"];
	        this.database = source["database"];
	        this.schema = source["schema"];
	    }
	}
	export class NotebookCellOutput {
	    stdout: string;
	    stderr: string;
	    error: string;
	    images: string[];
	    session_context?: NotebookSessionContext;
	
	    static createFrom(source: any = {}) {
	        return new NotebookCellOutput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stdout = source["stdout"];
	        this.stderr = source["stderr"];
	        this.error = source["error"];
	        this.images = source["images"];
	        this.session_context = this.convertValues(source["session_context"], NotebookSessionContext);
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
	export class NotebookCompletion {
	    label: string;
	    type: string;
	    detail: string;
	    documentation: string;
	
	    static createFrom(source: any = {}) {
	        return new NotebookCompletion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.type = source["type"];
	        this.detail = source["detail"];
	        this.documentation = source["documentation"];
	    }
	}
	
	export class NotebookSqlResult {
	    columns: string[];
	    rows: any[][];
	    rowCount: number;
	    queryID: string;
	
	    static createFrom(source: any = {}) {
	        return new NotebookSqlResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.rowCount = source["rowCount"];
	        this.queryID = source["queryID"];
	    }
	}
	export class NotebookSyntaxError {
	    severity: string;
	    line: number;
	    col: number;
	    endCol?: number;
	    msg: string;
	
	    static createFrom(source: any = {}) {
	        return new NotebookSyntaxError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.line = source["line"];
	        this.col = source["col"];
	        this.endCol = source["endCol"];
	        this.msg = source["msg"];
	    }
	}
	export class PackageInfo {
	    name: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new PackageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
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
	export class PythonInfo {
	    path: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new PythonInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.version = source["version"];
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
	export class SnowparkCheckResult {
	    isReady: boolean;
	    details: string;
	    pythonPath: string;
	    version: string;
	    systemPythonVersion: string;
	    backend: string;
	    venvPath: string;
	    hasConda: boolean;
	    hasEnv: boolean;
	    hasVenv: boolean;
	    hasSnowpark: boolean;
	    hasNotebook: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SnowparkCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isReady = source["isReady"];
	        this.details = source["details"];
	        this.pythonPath = source["pythonPath"];
	        this.version = source["version"];
	        this.systemPythonVersion = source["systemPythonVersion"];
	        this.backend = source["backend"];
	        this.venvPath = source["venvPath"];
	        this.hasConda = source["hasConda"];
	        this.hasEnv = source["hasEnv"];
	        this.hasVenv = source["hasVenv"];
	        this.hasSnowpark = source["hasSnowpark"];
	        this.hasNotebook = source["hasNotebook"];
	    }
	}
	export class SnowparkConfigResult {
	    backend: string;
	    venvPath: string;
	    pythonPath: string;
	
	    static createFrom(source: any = {}) {
	        return new SnowparkConfigResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backend = source["backend"];
	        this.venvPath = source["venvPath"];
	        this.pythonPath = source["pythonPath"];
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
	export class TableSummary {
	    name: string;
	    schema: string;
	    kind: string;
	    rows: number;
	    bytes: number;
	    owner: string;
	    retentionTime: number;
	    created: string;
	    lastAltered: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new TableSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.schema = source["schema"];
	        this.kind = source["kind"];
	        this.rows = source["rows"];
	        this.bytes = source["bytes"];
	        this.owner = source["owner"];
	        this.retentionTime = source["retentionTime"];
	        this.created = source["created"];
	        this.lastAltered = source["lastAltered"];
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
	
	export class ColumnInfo {
	    name: string;
	    dataType: string;
	    nullable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ColumnInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dataType = source["dataType"];
	        this.nullable = source["nullable"];
	    }
	}
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
	export class DataTypeInfo {
	    Name: string;
	    Kind: number;
	    ParamHint: string;
	
	    static createFrom(source: any = {}) {
	        return new DataTypeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Kind = source["Kind"];
	        this.ParamHint = source["ParamHint"];
	    }
	}
	export class DependencyNode {
	    name: string;
	    schema: string;
	    database: string;
	    kind: string;
	    children: DependencyNode[];
	    circular?: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DependencyNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.schema = source["schema"];
	        this.database = source["database"];
	        this.kind = source["kind"];
	        this.children = this.convertValues(source["children"], DependencyNode);
	        this.circular = source["circular"];
	        this.error = source["error"];
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
	export class DeployNotebookParams {
	    database: string;
	    schema: string;
	    name: string;
	    caseSensitive: boolean;
	    filePath: string;
	    content: string;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    comment: string;
	    queryWarehouse: string;
	    idleAutoShutdownSeconds: number;
	    runtimeName: string;
	    computePool: string;
	    warehouse: string;
	
	    static createFrom(source: any = {}) {
	        return new DeployNotebookParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.filePath = source["filePath"];
	        this.content = source["content"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.comment = source["comment"];
	        this.queryWarehouse = source["queryWarehouse"];
	        this.idleAutoShutdownSeconds = source["idleAutoShutdownSeconds"];
	        this.runtimeName = source["runtimeName"];
	        this.computePool = source["computePool"];
	        this.warehouse = source["warehouse"];
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
	export class FormatTypeOptions {
	    compression: string;
	    trimSpace: boolean;
	    replaceInvalidCharacters: boolean;
	    nullIf: string[];
	    dateFormat: string;
	    timeFormat: string;
	    timestampFormat: string;
	    binaryFormat: string;
	    fileExtension: string;
	    multiLine: boolean;
	    skipByteOrderMark: boolean;
	    ignoreUtf8Errors: boolean;
	    recordDelimiter: string;
	    fieldDelimiter: string;
	    parseHeader: boolean;
	    skipHeader: number;
	    skipBlankLines: boolean;
	    escape: string;
	    escapeUnenclosedField: string;
	    fieldOptionallyEnclosedBy: string;
	    errorOnColumnCountMismatch: boolean;
	    emptyFieldAsNull: boolean;
	    encoding: string;
	    enableOctal: boolean;
	    allowDuplicate: boolean;
	    stripOuterArray: boolean;
	    stripNullValues: boolean;
	    snappyCompression: boolean;
	    binaryAsText: boolean;
	    useLogicalType: boolean;
	    useVectorizedScanner: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FormatTypeOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.compression = source["compression"];
	        this.trimSpace = source["trimSpace"];
	        this.replaceInvalidCharacters = source["replaceInvalidCharacters"];
	        this.nullIf = source["nullIf"];
	        this.dateFormat = source["dateFormat"];
	        this.timeFormat = source["timeFormat"];
	        this.timestampFormat = source["timestampFormat"];
	        this.binaryFormat = source["binaryFormat"];
	        this.fileExtension = source["fileExtension"];
	        this.multiLine = source["multiLine"];
	        this.skipByteOrderMark = source["skipByteOrderMark"];
	        this.ignoreUtf8Errors = source["ignoreUtf8Errors"];
	        this.recordDelimiter = source["recordDelimiter"];
	        this.fieldDelimiter = source["fieldDelimiter"];
	        this.parseHeader = source["parseHeader"];
	        this.skipHeader = source["skipHeader"];
	        this.skipBlankLines = source["skipBlankLines"];
	        this.escape = source["escape"];
	        this.escapeUnenclosedField = source["escapeUnenclosedField"];
	        this.fieldOptionallyEnclosedBy = source["fieldOptionallyEnclosedBy"];
	        this.errorOnColumnCountMismatch = source["errorOnColumnCountMismatch"];
	        this.emptyFieldAsNull = source["emptyFieldAsNull"];
	        this.encoding = source["encoding"];
	        this.enableOctal = source["enableOctal"];
	        this.allowDuplicate = source["allowDuplicate"];
	        this.stripOuterArray = source["stripOuterArray"];
	        this.stripNullValues = source["stripNullValues"];
	        this.snappyCompression = source["snappyCompression"];
	        this.binaryAsText = source["binaryAsText"];
	        this.useLogicalType = source["useLogicalType"];
	        this.useVectorizedScanner = source["useVectorizedScanner"];
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
	    filePaths: string[];
	    format: string;
	    overwrite: boolean;
	    createTable: boolean;
	    options: FormatTypeOptions;
	
	    static createFrom(source: any = {}) {
	        return new ImportTableParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.filePaths = source["filePaths"];
	        this.format = source["format"];
	        this.overwrite = source["overwrite"];
	        this.createTable = source["createTable"];
	        this.options = this.convertValues(source["options"], FormatTypeOptions);
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
	export class IntegrationRow {
	    name: string;
	    type: string;
	    category: string;
	    enabled: boolean;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new IntegrationRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.category = source["category"];
	        this.enabled = source["enabled"];
	        this.comment = source["comment"];
	    }
	}
	
	export class QueryResult {
	    columns: string[];
	    rows: any[][];
	    rowsAffected: number;
	    queryID: string;
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.rowsAffected = source["rowsAffected"];
	        this.queryID = source["queryID"];
	        this.truncated = source["truncated"];
	    }
	}
	export class SchemaRef {
	    database: string;
	    schema: string;
	
	    static createFrom(source: any = {}) {
	        return new SchemaRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
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
	    predecessors?: string;
	    finalize?: string;
	
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
	        this.predecessors = source["predecessors"];
	        this.finalize = source["finalize"];
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
	export class TableForeignKey {
	    pkDatabase: string;
	    pkSchema: string;
	    pkTable: string;
	    pkColumn: string;
	    fkDatabase: string;
	    fkSchema: string;
	    fkTable: string;
	    fkColumn: string;
	    constraintName: string;
	    keySequence: number;
	
	    static createFrom(source: any = {}) {
	        return new TableForeignKey(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pkDatabase = source["pkDatabase"];
	        this.pkSchema = source["pkSchema"];
	        this.pkTable = source["pkTable"];
	        this.pkColumn = source["pkColumn"];
	        this.fkDatabase = source["fkDatabase"];
	        this.fkSchema = source["fkSchema"];
	        this.fkTable = source["fkTable"];
	        this.fkColumn = source["fkColumn"];
	        this.constraintName = source["constraintName"];
	        this.keySequence = source["keySequence"];
	    }
	}

}

export namespace sqleditor {
	
	export class ColInfo {
	    name: string;
	    dataType: string;
	
	    static createFrom(source: any = {}) {
	        return new ColInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dataType = source["dataType"];
	    }
	}
	export class ColEntry {
	    db: string;
	    schema: string;
	    name: string;
	    cols: ColInfo[];
	
	    static createFrom(source: any = {}) {
	        return new ColEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.db = source["db"];
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.cols = this.convertValues(source["cols"], ColInfo);
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
	
	export class DiagMarker {
	    startLineNumber: number;
	    startColumn: number;
	    endLineNumber: number;
	    endColumn: number;
	    message: string;
	    severity: number;
	
	    static createFrom(source: any = {}) {
	        return new DiagMarker(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startLineNumber = source["startLineNumber"];
	        this.startColumn = source["startColumn"];
	        this.endLineNumber = source["endLineNumber"];
	        this.endColumn = source["endColumn"];
	        this.message = source["message"];
	        this.severity = source["severity"];
	    }
	}
	export class FKEntry {
	    pkDatabase: string;
	    pkSchema: string;
	    pkTable: string;
	    pkColumn: string;
	    fkColumn: string;
	    constraintName: string;
	    keySequence: number;
	
	    static createFrom(source: any = {}) {
	        return new FKEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pkDatabase = source["pkDatabase"];
	        this.pkSchema = source["pkSchema"];
	        this.pkTable = source["pkTable"];
	        this.pkColumn = source["pkColumn"];
	        this.fkColumn = source["fkColumn"];
	        this.constraintName = source["constraintName"];
	        this.keySequence = source["keySequence"];
	    }
	}
	export class FunctionCallContext {
	    name: string;
	    paramIndex: number;
	
	    static createFrom(source: any = {}) {
	        return new FunctionCallContext(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.paramIndex = source["paramIndex"];
	    }
	}
	export class JoinCondition {
	    condition: string;
	    detail: string;
	    sortText: string;
	
	    static createFrom(source: any = {}) {
	        return new JoinCondition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.condition = source["condition"];
	        this.detail = source["detail"];
	        this.sortText = source["sortText"];
	    }
	}
	export class TableFKEntry {
	    db: string;
	    schema: string;
	    name: string;
	    fks: FKEntry[];
	
	    static createFrom(source: any = {}) {
	        return new TableFKEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.db = source["db"];
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.fks = this.convertValues(source["fks"], FKEntry);
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
	export class ResolvedRef {
	    alias: string;
	    db: string;
	    schema: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ResolvedRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alias = source["alias"];
	        this.db = source["db"];
	        this.schema = source["schema"];
	        this.name = source["name"];
	    }
	}
	export class JoinOnSuggestionsReq {
	    resolvedRefs: ResolvedRef[];
	    fkEntries: TableFKEntry[];
	    colEntries: ColEntry[];
	    prefix: string;
	
	    static createFrom(source: any = {}) {
	        return new JoinOnSuggestionsReq(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.resolvedRefs = this.convertValues(source["resolvedRefs"], ResolvedRef);
	        this.fkEntries = this.convertValues(source["fkEntries"], TableFKEntry);
	        this.colEntries = this.convertValues(source["colEntries"], ColEntry);
	        this.prefix = source["prefix"];
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
	export class JoinTableRef {
	    db: string;
	    schema: string;
	    name: string;
	    alias: string;
	
	    static createFrom(source: any = {}) {
	        return new JoinTableRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.db = source["db"];
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.alias = source["alias"];
	    }
	}
	
	export class SchemaEntry {
	    db: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new SchemaEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.db = source["db"];
	        this.name = source["name"];
	    }
	}
	export class ScriptingCompletionResult {
	    variables: string[];
	    needsColon: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScriptingCompletionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variables = source["variables"];
	        this.needsColon = source["needsColon"];
	    }
	}
	export class SignatureParam {
	    start: number;
	    end: number;
	
	    static createFrom(source: any = {}) {
	        return new SignatureParam(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = source["start"];
	        this.end = source["end"];
	    }
	}
	export class StatementRange {
	    startLine: number;
	    endLine: number;
	    startOffset: number;
	    endOffset: number;
	
	    static createFrom(source: any = {}) {
	        return new StatementRange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startLine = source["startLine"];
	        this.endLine = source["endLine"];
	        this.startOffset = source["startOffset"];
	        this.endOffset = source["endOffset"];
	    }
	}
	
	export class TokenMatch {
	    name: string;
	    line: number;
	    col: number;
	    endCol: number;
	    quoted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TokenMatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.line = source["line"];
	        this.col = source["col"];
	        this.endCol = source["endCol"];
	        this.quoted = source["quoted"];
	    }
	}
	export class ValidateBareColsRequest {
	    sql: string;
	    stmtRanges: StatementRange[];
	    resolvedRefs: ResolvedRef[];
	    colEntries: ColEntry[];
	    quotedIdentifiersIgnoreCase: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ValidateBareColsRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sql = source["sql"];
	        this.stmtRanges = this.convertValues(source["stmtRanges"], StatementRange);
	        this.resolvedRefs = this.convertValues(source["resolvedRefs"], ResolvedRef);
	        this.colEntries = this.convertValues(source["colEntries"], ColEntry);
	        this.quotedIdentifiersIgnoreCase = source["quotedIdentifiersIgnoreCase"];
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
	export class ValidateTablesExistRequest {
	    sql: string;
	    stmtRanges: StatementRange[];
	    resolvedRefs: ResolvedRef[];
	    knownDatabases: string[];
	    knownSchemas: SchemaEntry[];
	    quotedIdentifiersIgnoreCase: boolean;
	    droppedDatabases: string[];
	    droppedSchemas: SchemaEntry[];
	    droppedTables: ResolvedRef[];
	
	    static createFrom(source: any = {}) {
	        return new ValidateTablesExistRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sql = source["sql"];
	        this.stmtRanges = this.convertValues(source["stmtRanges"], StatementRange);
	        this.resolvedRefs = this.convertValues(source["resolvedRefs"], ResolvedRef);
	        this.knownDatabases = source["knownDatabases"];
	        this.knownSchemas = this.convertValues(source["knownSchemas"], SchemaEntry);
	        this.quotedIdentifiersIgnoreCase = source["quotedIdentifiersIgnoreCase"];
	        this.droppedDatabases = source["droppedDatabases"];
	        this.droppedSchemas = this.convertValues(source["droppedSchemas"], SchemaEntry);
	        this.droppedTables = this.convertValues(source["droppedTables"], ResolvedRef);
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

export namespace tasks {
	
	export class FinalizabilityRow {
	    name: string;
	    disabledReason: string;
	
	    static createFrom(source: any = {}) {
	        return new FinalizabilityRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.disabledReason = source["disabledReason"];
	    }
	}
	export class StatusRow {
	    name: string;
	    taskState: string;
	    predecessors: string;
	    lastRunState: string;
	    lastRunTime: string;
	    errorMsg: string;
	    finalize: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.taskState = source["taskState"];
	        this.predecessors = source["predecessors"];
	        this.lastRunState = source["lastRunState"];
	        this.lastRunTime = source["lastRunTime"];
	        this.errorMsg = source["errorMsg"];
	        this.finalize = source["finalize"];
	    }
	}
	export class StatusesResult {
	    rows: StatusRow[];
	    historyError: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = this.convertValues(source["rows"], StatusRow);
	        this.historyError = source["historyError"];
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

