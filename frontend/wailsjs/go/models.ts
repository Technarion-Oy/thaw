export namespace aggregationpolicy {
	
	export class AggregationPolicyConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    body: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new AggregationPolicyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.body = source["body"];
	        this.comment = source["comment"];
	    }
	}

}

export namespace alert {
	
	export class AlertConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    warehouse: string;
	    schedule: string;
	    comment: string;
	    tags: snowflake.TagPair[];
	    condition: string;
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new AlertConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.warehouse = source["warehouse"];
	        this.schedule = source["schedule"];
	        this.comment = source["comment"];
	        this.tags = this.convertValues(source["tags"], snowflake.TagPair);
	        this.condition = source["condition"];
	        this.action = source["action"];
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

export namespace app {
	
	export class AppInfo {
	    companyName: string;
	    productName: string;
	    productVersion: string;
	    copyright: string;
	    comments: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.companyName = source["companyName"];
	        this.productName = source["productName"];
	        this.productVersion = source["productVersion"];
	        this.copyright = source["copyright"];
	        this.comments = source["comments"];
	    }
	}

}

export namespace authenticationpolicy {
	
	export class AuthenticationPolicyConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    authenticationMethods: string[];
	    clientTypes: string[];
	    securityIntegrations: string[];
	    mfaEnrollment: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthenticationPolicyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.authenticationMethods = source["authenticationMethods"];
	        this.clientTypes = source["clientTypes"];
	        this.securityIntegrations = source["securityIntegrations"];
	        this.mfaEnrollment = source["mfaEnrollment"];
	        this.comment = source["comment"];
	    }
	}
	export class ClientPolicyEntry {
	    driver: string;
	    minimumVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new ClientPolicyEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.driver = source["driver"];
	        this.minimumVersion = source["minimumVersion"];
	    }
	}
	export class ClientPolicy {
	    entries: ClientPolicyEntry[];
	
	    static createFrom(source: any = {}) {
	        return new ClientPolicy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entries = this.convertValues(source["entries"], ClientPolicyEntry);
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
	
	export class DriverVersionHint {
	    driver: string;
	    minimumSupported: string;
	    recommended: string;
	
	    static createFrom(source: any = {}) {
	        return new DriverVersionHint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.driver = source["driver"];
	        this.minimumSupported = source["minimumSupported"];
	        this.recommended = source["recommended"];
	    }
	}
	export class ListParamMeta {
	    keyword: string;
	    label: string;
	    options: string[];
	    freeform: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ListParamMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keyword = source["keyword"];
	        this.label = source["label"];
	        this.options = source["options"];
	        this.freeform = source["freeform"];
	    }
	}
	export class MFAPolicy {
	    allowedMethods: string[];
	    enforceMfaOnExternalAuthentication: string;
	
	    static createFrom(source: any = {}) {
	        return new MFAPolicy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allowedMethods = source["allowedMethods"];
	        this.enforceMfaOnExternalAuthentication = source["enforceMfaOnExternalAuthentication"];
	    }
	}
	export class PATPolicy {
	    defaultExpiryInDays?: number;
	    maxExpiryInDays?: number;
	    networkPolicyEvaluation: string;
	    requireRoleRestrictionForServiceUsers?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PATPolicy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultExpiryInDays = source["defaultExpiryInDays"];
	        this.maxExpiryInDays = source["maxExpiryInDays"];
	        this.networkPolicyEvaluation = source["networkPolicyEvaluation"];
	        this.requireRoleRestrictionForServiceUsers = source["requireRoleRestrictionForServiceUsers"];
	    }
	}
	export class WorkloadIdentityPolicy {
	    allowedProviders: string[];
	    allowedAwsAccounts: string[];
	    allowedAzureIssuers: string[];
	    allowedOidcIssuers: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorkloadIdentityPolicy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allowedProviders = source["allowedProviders"];
	        this.allowedAwsAccounts = source["allowedAwsAccounts"];
	        this.allowedAzureIssuers = source["allowedAzureIssuers"];
	        this.allowedOidcIssuers = source["allowedOidcIssuers"];
	    }
	}

}

export namespace backup {
	
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

}

export namespace column {
	
	export class AddColumnConfig {
	    name: string;
	    caseSensitive: boolean;
	    ifNotExists: boolean;
	    dataType: string;
	    valueMode: string;
	    defaultValue: string;
	    computedExpr: string;
	    identityStart: number;
	    identityStep: number;
	    identityOrder: string;
	    notNull: boolean;
	    constraintKind: string;
	    constraintName: string;
	    fkDb: string;
	    fkSchema: string;
	    fkTableName: string;
	    fkColumn: string;
	    collation: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new AddColumnConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.ifNotExists = source["ifNotExists"];
	        this.dataType = source["dataType"];
	        this.valueMode = source["valueMode"];
	        this.defaultValue = source["defaultValue"];
	        this.computedExpr = source["computedExpr"];
	        this.identityStart = source["identityStart"];
	        this.identityStep = source["identityStep"];
	        this.identityOrder = source["identityOrder"];
	        this.notNull = source["notNull"];
	        this.constraintKind = source["constraintKind"];
	        this.constraintName = source["constraintName"];
	        this.fkDb = source["fkDb"];
	        this.fkSchema = source["fkSchema"];
	        this.fkTableName = source["fkTableName"];
	        this.fkColumn = source["fkColumn"];
	        this.collation = source["collation"];
	        this.comment = source["comment"];
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
	    version: number;
	    resultsetExport: boolean;
	    exportTableData: boolean;
	    tableDataImport: boolean;
	    ddlExport: boolean;
	    putCommand: boolean;
	    getCommand: boolean;
	    removeCommand: boolean;
	    userRoleManagement: boolean;
	    warehouseManagement: boolean;
	    warehouseCreditUsage: boolean;
	    queryActivityHistory: boolean;
	    integrationsManagement: boolean;
	    backupPoliciesAndSets: boolean;
	    aiInlineCompletions: boolean;
	    schemaMigration: boolean;
	    dbtScaffolding: boolean;
	    dbtProjectBrowser: boolean;
	    erDiagramDesigner: boolean;
	    taskGraphVisualizer: boolean;
	    insertMapping: boolean;
	    codeSnippets: boolean;
	    snowparkNotebooks: boolean;
	    embeddedTerminal: boolean;
	    gitIntegration: boolean;
	    queryProfile: boolean;
	    explainSql: boolean;
	    queryLog: boolean;
	    sqlDiagnostics: boolean;
	    schemaAutocomplete: boolean;
	    ddlHoverTooltips: boolean;
	    fileFormatBuilder: boolean;
	    snowflakeCLIProfileManager: boolean;
	    multiCellCopy: boolean;
	    cellDetailPanel: boolean;
	    columnReorder: boolean;
	    crossTabSearch: boolean;
	    fileWatcher: boolean;
	    columnManagement: boolean;
	    mcpServer: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FeatureFlags(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.initialized = source["initialized"];
	        this.version = source["version"];
	        this.resultsetExport = source["resultsetExport"];
	        this.exportTableData = source["exportTableData"];
	        this.tableDataImport = source["tableDataImport"];
	        this.ddlExport = source["ddlExport"];
	        this.putCommand = source["putCommand"];
	        this.getCommand = source["getCommand"];
	        this.removeCommand = source["removeCommand"];
	        this.userRoleManagement = source["userRoleManagement"];
	        this.warehouseManagement = source["warehouseManagement"];
	        this.warehouseCreditUsage = source["warehouseCreditUsage"];
	        this.queryActivityHistory = source["queryActivityHistory"];
	        this.integrationsManagement = source["integrationsManagement"];
	        this.backupPoliciesAndSets = source["backupPoliciesAndSets"];
	        this.aiInlineCompletions = source["aiInlineCompletions"];
	        this.schemaMigration = source["schemaMigration"];
	        this.dbtScaffolding = source["dbtScaffolding"];
	        this.dbtProjectBrowser = source["dbtProjectBrowser"];
	        this.erDiagramDesigner = source["erDiagramDesigner"];
	        this.taskGraphVisualizer = source["taskGraphVisualizer"];
	        this.insertMapping = source["insertMapping"];
	        this.codeSnippets = source["codeSnippets"];
	        this.snowparkNotebooks = source["snowparkNotebooks"];
	        this.embeddedTerminal = source["embeddedTerminal"];
	        this.gitIntegration = source["gitIntegration"];
	        this.queryProfile = source["queryProfile"];
	        this.explainSql = source["explainSql"];
	        this.queryLog = source["queryLog"];
	        this.sqlDiagnostics = source["sqlDiagnostics"];
	        this.schemaAutocomplete = source["schemaAutocomplete"];
	        this.ddlHoverTooltips = source["ddlHoverTooltips"];
	        this.fileFormatBuilder = source["fileFormatBuilder"];
	        this.snowflakeCLIProfileManager = source["snowflakeCLIProfileManager"];
	        this.multiCellCopy = source["multiCellCopy"];
	        this.cellDetailPanel = source["cellDetailPanel"];
	        this.columnReorder = source["columnReorder"];
	        this.crossTabSearch = source["crossTabSearch"];
	        this.fileWatcher = source["fileWatcher"];
	        this.columnManagement = source["columnManagement"];
	        this.mcpServer = source["mcpServer"];
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
	export class PipRegistryCredential {
	    registry: string;
	    username: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new PipRegistryCredential(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.registry = source["registry"];
	        this.username = source["username"];
	        this.password = source["password"];
	    }
	}
	export class PipRegistryConfig {
	    primaryURL: string;
	    additionalRegistries: string[];
	    behavior: string;
	    credentials: PipRegistryCredential[];
	    enableProxy: boolean;
	    proxyURL: string;
	    proxyUsername: string;
	    proxyPassword: string;
	    proxyBypassHosts: string;
	    trustedHosts: string;
	    customCACertPath: string;
	
	    static createFrom(source: any = {}) {
	        return new PipRegistryConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.primaryURL = source["primaryURL"];
	        this.additionalRegistries = source["additionalRegistries"];
	        this.behavior = source["behavior"];
	        this.credentials = this.convertValues(source["credentials"], PipRegistryCredential);
	        this.enableProxy = source["enableProxy"];
	        this.proxyURL = source["proxyURL"];
	        this.proxyUsername = source["proxyUsername"];
	        this.proxyPassword = source["proxyPassword"];
	        this.proxyBypassHosts = source["proxyBypassHosts"];
	        this.trustedHosts = source["trustedHosts"];
	        this.customCACertPath = source["customCACertPath"];
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
	
	export class SessionConfig {
	    maxSessions: number;
	    maxOpenConnsPerSession: number;
	    maxIdleConnsPerSession: number;
	    initMode: string;
	    idleTimeoutMinutes: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxSessions = source["maxSessions"];
	        this.maxOpenConnsPerSession = source["maxOpenConnsPerSession"];
	        this.maxIdleConnsPerSession = source["maxIdleConnsPerSession"];
	        this.initMode = source["initMode"];
	        this.idleTimeoutMinutes = source["idleTimeoutMinutes"];
	    }
	}

}

export namespace datametricfunction {
	
	export class DataMetricFunctionColumn {
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new DataMetricFunctionColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	export class DataMetricFunctionTableArg {
	    name: string;
	    columns: DataMetricFunctionColumn[];
	
	    static createFrom(source: any = {}) {
	        return new DataMetricFunctionTableArg(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = this.convertValues(source["columns"], DataMetricFunctionColumn);
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
	export class DataMetricFunctionConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    secure: boolean;
	    args: DataMetricFunctionTableArg[];
	    notNull: boolean;
	    comment: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new DataMetricFunctionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.secure = source["secure"];
	        this.args = this.convertValues(source["args"], DataMetricFunctionTableArg);
	        this.notNull = source["notNull"];
	        this.comment = source["comment"];
	        this.body = source["body"];
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

export namespace dbtproject {
	
	export class AlterSetConfig {
	    dbtVersion: string;
	    defaultTarget: string;
	    externalAccessIntegrations: string[];
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new AlterSetConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dbtVersion = source["dbtVersion"];
	        this.defaultTarget = source["defaultTarget"];
	        this.externalAccessIntegrations = source["externalAccessIntegrations"];
	        this.comment = source["comment"];
	    }
	}
	export class CreateConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    sourceLocation: string;
	    comment: string;
	    dbtVersion: string;
	    defaultTarget: string;
	    externalAccessIntegrations: string[];
	
	    static createFrom(source: any = {}) {
	        return new CreateConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.sourceLocation = source["sourceLocation"];
	        this.comment = source["comment"];
	        this.dbtVersion = source["dbtVersion"];
	        this.defaultTarget = source["defaultTarget"];
	        this.externalAccessIntegrations = source["externalAccessIntegrations"];
	    }
	}
	export class DbtVersionInfo {
	    dbt_version: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new DbtVersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dbt_version = source["dbt_version"];
	        this.type = source["type"];
	    }
	}
	export class ExecuteConfig {
	    args: string;
	    dbtVersion: string;
	    fromWorkspace: string;
	    projectRoot: string;
	
	    static createFrom(source: any = {}) {
	        return new ExecuteConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.args = source["args"];
	        this.dbtVersion = source["dbtVersion"];
	        this.fromWorkspace = source["fromWorkspace"];
	        this.projectRoot = source["projectRoot"];
	    }
	}

}

export namespace ddl {
	
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

export namespace dynamictable {
	
	export class DynamicTableConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    transient: boolean;
	    targetLag: string;
	    scheduler: string;
	    warehouse: string;
	    initializationWarehouse: string;
	    refreshMode: string;
	    initialize: string;
	    clusterBy: string;
	    dataRetentionTimeInDays: string;
	    maxDataExtensionTimeInDays: string;
	    comment: string;
	    copyGrants: boolean;
	    requireUser: boolean;
	    rowTimestamp: string;
	    tags: snowflake.TagPair[];
	    query: string;
	
	    static createFrom(source: any = {}) {
	        return new DynamicTableConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.transient = source["transient"];
	        this.targetLag = source["targetLag"];
	        this.scheduler = source["scheduler"];
	        this.warehouse = source["warehouse"];
	        this.initializationWarehouse = source["initializationWarehouse"];
	        this.refreshMode = source["refreshMode"];
	        this.initialize = source["initialize"];
	        this.clusterBy = source["clusterBy"];
	        this.dataRetentionTimeInDays = source["dataRetentionTimeInDays"];
	        this.maxDataExtensionTimeInDays = source["maxDataExtensionTimeInDays"];
	        this.comment = source["comment"];
	        this.copyGrants = source["copyGrants"];
	        this.requireUser = source["requireUser"];
	        this.rowTimestamp = source["rowTimestamp"];
	        this.tags = this.convertValues(source["tags"], snowflake.TagPair);
	        this.query = source["query"];
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

export namespace erdesigner {
	
	export class FKColRef {
	    schema: string;
	    table: string;
	    col: string;
	
	    static createFrom(source: any = {}) {
	        return new FKColRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.col = source["col"];
	    }
	}
	export class FKPair {
	    from: FKColRef;
	    to: FKColRef;
	
	    static createFrom(source: any = {}) {
	        return new FKPair(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = this.convertValues(source["from"], FKColRef);
	        this.to = this.convertValues(source["to"], FKColRef);
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
	export class TableRef {
	    schema: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new TableRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	    }
	}
	export class JoinEntry {
	    table: TableRef;
	    joinType: string;
	    onCondition: string;
	    fkPairs: FKPair[];
	    isIntermediate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new JoinEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.table = this.convertValues(source["table"], TableRef);
	        this.joinType = source["joinType"];
	        this.onCondition = source["onCondition"];
	        this.fkPairs = this.convertValues(source["fkPairs"], FKPair);
	        this.isIntermediate = source["isIntermediate"];
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
	export class JoinPathEdge {
	    from: FKColRef;
	    to: FKColRef;
	
	    static createFrom(source: any = {}) {
	        return new JoinPathEdge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = this.convertValues(source["from"], FKColRef);
	        this.to = this.convertValues(source["to"], FKColRef);
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
	export class JoinPath {
	    tables: TableRef[];
	    edges: JoinPathEdge[];
	
	    static createFrom(source: any = {}) {
	        return new JoinPath(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tables = this.convertValues(source["tables"], TableRef);
	        this.edges = this.convertValues(source["edges"], JoinPathEdge);
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
	
	export class JoinQueryState {
	    database: string;
	    baseTable: TableRef;
	    joins: JoinEntry[];
	    selectedColumns: Record<string, Array<string>>;
	
	    static createFrom(source: any = {}) {
	        return new JoinQueryState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.baseTable = this.convertValues(source["baseTable"], TableRef);
	        this.joins = this.convertValues(source["joins"], JoinEntry);
	        this.selectedColumns = source["selectedColumns"];
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

export namespace eventtable {
	
	export class EventTableConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    clusterBy: string;
	    dataRetentionTimeInDays: string;
	    maxDataExtensionTimeInDays: string;
	    changeTracking: string;
	    defaultDdlCollation: string;
	    copyGrants: boolean;
	    comment: string;
	    tags: snowflake.TagPair[];
	
	    static createFrom(source: any = {}) {
	        return new EventTableConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.clusterBy = source["clusterBy"];
	        this.dataRetentionTimeInDays = source["dataRetentionTimeInDays"];
	        this.maxDataExtensionTimeInDays = source["maxDataExtensionTimeInDays"];
	        this.changeTracking = source["changeTracking"];
	        this.defaultDdlCollation = source["defaultDdlCollation"];
	        this.copyGrants = source["copyGrants"];
	        this.comment = source["comment"];
	        this.tags = this.convertValues(source["tags"], snowflake.TagPair);
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

export namespace externalfunction {
	
	export class BuilderOptions {
	    compression: string[];
	    nullHandling: string[];
	    volatility: string[];
	    contextHeaders: string[];
	
	    static createFrom(source: any = {}) {
	        return new BuilderOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.compression = source["compression"];
	        this.nullHandling = source["nullHandling"];
	        this.volatility = source["volatility"];
	        this.contextHeaders = source["contextHeaders"];
	    }
	}
	export class ExternalFunctionArg {
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new ExternalFunctionArg(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	export class HeaderPair {
	    name: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new HeaderPair(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	    }
	}
	export class ExternalFunctionConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    secure: boolean;
	    args: ExternalFunctionArg[];
	    returns: string;
	    notNull: boolean;
	    nullHandling: string;
	    volatility: string;
	    comment: string;
	    apiIntegration: string;
	    headers: HeaderPair[];
	    contextHeaders: string[];
	    maxBatchRows: string;
	    compression: string;
	    requestTranslator: string;
	    responseTranslator: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new ExternalFunctionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.secure = source["secure"];
	        this.args = this.convertValues(source["args"], ExternalFunctionArg);
	        this.returns = source["returns"];
	        this.notNull = source["notNull"];
	        this.nullHandling = source["nullHandling"];
	        this.volatility = source["volatility"];
	        this.comment = source["comment"];
	        this.apiIntegration = source["apiIntegration"];
	        this.headers = this.convertValues(source["headers"], HeaderPair);
	        this.contextHeaders = source["contextHeaders"];
	        this.maxBatchRows = source["maxBatchRows"];
	        this.compression = source["compression"];
	        this.requestTranslator = source["requestTranslator"];
	        this.responseTranslator = source["responseTranslator"];
	        this.url = source["url"];
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

export namespace externaltable {
	
	export class ExternalTableColumn {
	    name: string;
	    type: string;
	    expression: string;
	    partition: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExternalTableColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.expression = source["expression"];
	        this.partition = source["partition"];
	    }
	}
	export class ExternalTableConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    columns: ExternalTableColumn[];
	    location: string;
	    refreshOnCreate: string;
	    autoRefresh: string;
	    pattern: string;
	    fileFormatName: string;
	    fileFormatType: string;
	    awsSnsTopic: string;
	    copyGrants: boolean;
	    comment: string;
	    tags: snowflake.TagPair[];
	
	    static createFrom(source: any = {}) {
	        return new ExternalTableConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.columns = this.convertValues(source["columns"], ExternalTableColumn);
	        this.location = source["location"];
	        this.refreshOnCreate = source["refreshOnCreate"];
	        this.autoRefresh = source["autoRefresh"];
	        this.pattern = source["pattern"];
	        this.fileFormatName = source["fileFormatName"];
	        this.fileFormatType = source["fileFormatType"];
	        this.awsSnsTopic = source["awsSnsTopic"];
	        this.copyGrants = source["copyGrants"];
	        this.comment = source["comment"];
	        this.tags = this.convertValues(source["tags"], snowflake.TagPair);
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

export namespace fileformat {
	
	export class FileFormatConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    type: string;
	    comment: string;
	    compression: string;
	    trimSpace: boolean;
	    replaceInvalid: boolean;
	    fileExtension: string;
	    recordDelimiter: string;
	    fieldDelimiter: string;
	    multiLine: boolean;
	    parseHeader: boolean;
	    skipHeader: number;
	    skipBlankLines: boolean;
	    dateFormat: string;
	    timeFormat: string;
	    timestampFormat: string;
	    binaryFormat: string;
	    escape: string;
	    escapeUnenclosedField: string;
	    fieldOptionallyEnclosedBy: string;
	    nullIf: string[];
	    errorOnColumnCountMismatch: boolean;
	    emptyFieldAsNull: boolean;
	    skipByteOrderMark: boolean;
	    encoding: string;
	    enableOctal: boolean;
	    allowDuplicate: boolean;
	    stripOuterArray: boolean;
	    stripNullValues: boolean;
	    ignoreUTF8Errors: boolean;
	    preserveSpace: boolean;
	    stripOuterElement: boolean;
	    disableSnowflakeData: boolean;
	    disableAutoConvert: boolean;
	    binaryAsText: boolean;
	    useLogicalType: boolean;
	    snappyCompression: boolean;
	    snappyCompressionLevel: number;
	    useVectorizedScanner: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileFormatConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.type = source["type"];
	        this.comment = source["comment"];
	        this.compression = source["compression"];
	        this.trimSpace = source["trimSpace"];
	        this.replaceInvalid = source["replaceInvalid"];
	        this.fileExtension = source["fileExtension"];
	        this.recordDelimiter = source["recordDelimiter"];
	        this.fieldDelimiter = source["fieldDelimiter"];
	        this.multiLine = source["multiLine"];
	        this.parseHeader = source["parseHeader"];
	        this.skipHeader = source["skipHeader"];
	        this.skipBlankLines = source["skipBlankLines"];
	        this.dateFormat = source["dateFormat"];
	        this.timeFormat = source["timeFormat"];
	        this.timestampFormat = source["timestampFormat"];
	        this.binaryFormat = source["binaryFormat"];
	        this.escape = source["escape"];
	        this.escapeUnenclosedField = source["escapeUnenclosedField"];
	        this.fieldOptionallyEnclosedBy = source["fieldOptionallyEnclosedBy"];
	        this.nullIf = source["nullIf"];
	        this.errorOnColumnCountMismatch = source["errorOnColumnCountMismatch"];
	        this.emptyFieldAsNull = source["emptyFieldAsNull"];
	        this.skipByteOrderMark = source["skipByteOrderMark"];
	        this.encoding = source["encoding"];
	        this.enableOctal = source["enableOctal"];
	        this.allowDuplicate = source["allowDuplicate"];
	        this.stripOuterArray = source["stripOuterArray"];
	        this.stripNullValues = source["stripNullValues"];
	        this.ignoreUTF8Errors = source["ignoreUTF8Errors"];
	        this.preserveSpace = source["preserveSpace"];
	        this.stripOuterElement = source["stripOuterElement"];
	        this.disableSnowflakeData = source["disableSnowflakeData"];
	        this.disableAutoConvert = source["disableAutoConvert"];
	        this.binaryAsText = source["binaryAsText"];
	        this.useLogicalType = source["useLogicalType"];
	        this.snappyCompression = source["snappyCompression"];
	        this.snappyCompressionLevel = source["snappyCompressionLevel"];
	        this.useVectorizedScanner = source["useVectorizedScanner"];
	    }
	}
	export class PreviewResult {
	    columns: string[];
	    rows: any[];
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new PreviewResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.error = source["error"];
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
	
	export class BranchInfo {
	    name: string;
	    isRemote: boolean;
	    isCurrent: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BranchInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.isRemote = source["isRemote"];
	        this.isCurrent = source["isCurrent"];
	    }
	}
	export class CloneParams {
	    url: string;
	    path: string;
	    authMethod: string;
	    token: string;
	
	    static createFrom(source: any = {}) {
	        return new CloneParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.path = source["path"];
	        this.authMethod = source["authMethod"];
	        this.token = source["token"];
	    }
	}
	export class CredentialResult {
	    found: boolean;
	    username: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new CredentialResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.username = source["username"];
	        this.source = source["source"];
	    }
	}
	export class PullParams {
	    dir: string;
	    remoteURL: string;
	    branch: string;
	    authMethod: string;
	    token: string;
	
	    static createFrom(source: any = {}) {
	        return new PullParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.remoteURL = source["remoteURL"];
	        this.branch = source["branch"];
	        this.authMethod = source["authMethod"];
	        this.token = source["token"];
	    }
	}
	export class PushParams {
	    dir: string;
	    remoteURL: string;
	    branch: string;
	    authMethod: string;
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
	        this.authMethod = source["authMethod"];
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
	    totalChanged: number;
	
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
	        this.totalChanged = source["totalChanged"];
	    }
	}

}

export namespace hybridtable {
	
	export class HybridColumn {
	    name: string;
	    type: string;
	    notNull: boolean;
	    primaryKey: boolean;
	    default: string;
	
	    static createFrom(source: any = {}) {
	        return new HybridColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.notNull = source["notNull"];
	        this.primaryKey = source["primaryKey"];
	        this.default = source["default"];
	    }
	}
	export class HybridIndex {
	    name: string;
	    columns: string[];
	    include: string[];
	
	    static createFrom(source: any = {}) {
	        return new HybridIndex(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	        this.include = source["include"];
	    }
	}
	export class HybridTableConfig {
	    name: string;
	    caseSensitive: boolean;
	    ifNotExists: boolean;
	    columns: HybridColumn[];
	    indexes: HybridIndex[];
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new HybridTableConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.ifNotExists = source["ifNotExists"];
	        this.columns = this.convertValues(source["columns"], HybridColumn);
	        this.indexes = this.convertValues(source["indexes"], HybridIndex);
	        this.comment = source["comment"];
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
	export class IndexColumn {
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new IndexColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	export class IndexColumnOptions {
	    keyColumns: string[];
	    includeColumns: string[];
	
	    static createFrom(source: any = {}) {
	        return new IndexColumnOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keyColumns = source["keyColumns"];
	        this.includeColumns = source["includeColumns"];
	    }
	}

}

export namespace icebergtable {
	
	export class IcebergColumn {
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new IcebergColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	export class IcebergTableConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    tableType: string;
	    columns: IcebergColumn[];
	    externalVolume: string;
	    catalog: string;
	    baseLocation: string;
	    catalogTableName: string;
	    catalogNamespace: string;
	    metadataFilePath: string;
	    replaceInvalidCharacters: string;
	    autoRefresh: string;
	    clusterBy: string;
	    comment: string;
	    tags: snowflake.TagPair[];
	
	    static createFrom(source: any = {}) {
	        return new IcebergTableConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.tableType = source["tableType"];
	        this.columns = this.convertValues(source["columns"], IcebergColumn);
	        this.externalVolume = source["externalVolume"];
	        this.catalog = source["catalog"];
	        this.baseLocation = source["baseLocation"];
	        this.catalogTableName = source["catalogTableName"];
	        this.catalogNamespace = source["catalogNamespace"];
	        this.metadataFilePath = source["metadataFilePath"];
	        this.replaceInvalidCharacters = source["replaceInvalidCharacters"];
	        this.autoRefresh = source["autoRefresh"];
	        this.clusterBy = source["clusterBy"];
	        this.comment = source["comment"];
	        this.tags = this.convertValues(source["tags"], snowflake.TagPair);
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

export namespace imagerepository {
	
	export class ImageRepositoryConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new ImageRepositoryConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.comment = source["comment"];
	    }
	}

}

export namespace integrations {
	
	export class ApiIntegrationParams {
	    name: string;
	    caseSensitive: boolean;
	    enabled: boolean;
	    provider: string;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    allowedPrefixes: string;
	    blockedPrefixes: string;
	    awsRoleArn: string;
	    apiKey: string;
	    azureTenantId: string;
	    azureAdAppId: string;
	    googleAudience: string;
	    gitAuthMode: string;
	    githubAppPath: string;
	    allowedAuthSecrets: string[];
	    oauthClientId: string;
	    oauthClientSecret: string;
	    oauthTokenEndpoint: string;
	    oauthScopes: string;
	    usePrivateLink: boolean;
	    tlsCertificates: string[];
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new ApiIntegrationParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.enabled = source["enabled"];
	        this.provider = source["provider"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.allowedPrefixes = source["allowedPrefixes"];
	        this.blockedPrefixes = source["blockedPrefixes"];
	        this.awsRoleArn = source["awsRoleArn"];
	        this.apiKey = source["apiKey"];
	        this.azureTenantId = source["azureTenantId"];
	        this.azureAdAppId = source["azureAdAppId"];
	        this.googleAudience = source["googleAudience"];
	        this.gitAuthMode = source["gitAuthMode"];
	        this.githubAppPath = source["githubAppPath"];
	        this.allowedAuthSecrets = source["allowedAuthSecrets"];
	        this.oauthClientId = source["oauthClientId"];
	        this.oauthClientSecret = source["oauthClientSecret"];
	        this.oauthTokenEndpoint = source["oauthTokenEndpoint"];
	        this.oauthScopes = source["oauthScopes"];
	        this.usePrivateLink = source["usePrivateLink"];
	        this.tlsCertificates = source["tlsCertificates"];
	        this.comment = source["comment"];
	    }
	}
	export class CatalogIntegrationParams {
	    name: string;
	    caseSensitive: boolean;
	    enabled: boolean;
	    source: string;
	    glueAwsRoleArn: string;
	    glueCatalogId: string;
	    glueRegion: string;
	    tableFormat: string;
	    catalogUri: string;
	    catalogName: string;
	    catalogNamespace: string;
	    catalogApiType: string;
	    accessDelegationMode: string;
	    prefix: string;
	    oauthTokenUri: string;
	    oauthClientId: string;
	    oauthClientSecret: string;
	    oauthScopes: string;
	    icebergAuthType: string;
	    bearerToken: string;
	    sapInvitationLink: string;
	    refreshInterval: number;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new CatalogIntegrationParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.enabled = source["enabled"];
	        this.source = source["source"];
	        this.glueAwsRoleArn = source["glueAwsRoleArn"];
	        this.glueCatalogId = source["glueCatalogId"];
	        this.glueRegion = source["glueRegion"];
	        this.tableFormat = source["tableFormat"];
	        this.catalogUri = source["catalogUri"];
	        this.catalogName = source["catalogName"];
	        this.catalogNamespace = source["catalogNamespace"];
	        this.catalogApiType = source["catalogApiType"];
	        this.accessDelegationMode = source["accessDelegationMode"];
	        this.prefix = source["prefix"];
	        this.oauthTokenUri = source["oauthTokenUri"];
	        this.oauthClientId = source["oauthClientId"];
	        this.oauthClientSecret = source["oauthClientSecret"];
	        this.oauthScopes = source["oauthScopes"];
	        this.icebergAuthType = source["icebergAuthType"];
	        this.bearerToken = source["bearerToken"];
	        this.sapInvitationLink = source["sapInvitationLink"];
	        this.refreshInterval = source["refreshInterval"];
	        this.comment = source["comment"];
	    }
	}
	export class ExternalAccessIntegrationParams {
	    name: string;
	    caseSensitive: boolean;
	    enabled: boolean;
	    allowedNetworkRules: string;
	    allowedApiAuthIntegrations: string;
	    allowedAuthSecrets: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new ExternalAccessIntegrationParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.enabled = source["enabled"];
	        this.allowedNetworkRules = source["allowedNetworkRules"];
	        this.allowedApiAuthIntegrations = source["allowedApiAuthIntegrations"];
	        this.allowedAuthSecrets = source["allowedAuthSecrets"];
	        this.comment = source["comment"];
	    }
	}
	export class NotificationIntegrationParams {
	    name: string;
	    caseSensitive: boolean;
	    enabled: boolean;
	    subtype: string;
	    azureQueueUri: string;
	    azureTenantId: string;
	    usePrivatelink: boolean;
	    gcpSubName: string;
	    awsSnsTopicArn: string;
	    awsSnsRoleArn: string;
	    azureTopicEndpoint: string;
	    gcpTopicName: string;
	    allowedRecipients: string;
	    defaultRecipients: string;
	    defaultSubject: string;
	    webhookUrl: string;
	    webhookSecret: string;
	    webhookBodyTemplate: string;
	    webhookHeaders: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new NotificationIntegrationParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.enabled = source["enabled"];
	        this.subtype = source["subtype"];
	        this.azureQueueUri = source["azureQueueUri"];
	        this.azureTenantId = source["azureTenantId"];
	        this.usePrivatelink = source["usePrivatelink"];
	        this.gcpSubName = source["gcpSubName"];
	        this.awsSnsTopicArn = source["awsSnsTopicArn"];
	        this.awsSnsRoleArn = source["awsSnsRoleArn"];
	        this.azureTopicEndpoint = source["azureTopicEndpoint"];
	        this.gcpTopicName = source["gcpTopicName"];
	        this.allowedRecipients = source["allowedRecipients"];
	        this.defaultRecipients = source["defaultRecipients"];
	        this.defaultSubject = source["defaultSubject"];
	        this.webhookUrl = source["webhookUrl"];
	        this.webhookSecret = source["webhookSecret"];
	        this.webhookBodyTemplate = source["webhookBodyTemplate"];
	        this.webhookHeaders = source["webhookHeaders"];
	        this.comment = source["comment"];
	    }
	}
	export class SecurityIntegrationParams {
	    name: string;
	    caseSensitive: boolean;
	    enabled: boolean;
	    secType: string;
	    authType: string;
	    awsRoleArn: string;
	    oauthGrant: string;
	    oauthTokenEndpoint: string;
	    oauthClientId: string;
	    oauthClientSecret: string;
	    oauthScopes: string;
	    externalOauthType: string;
	    issuer: string;
	    tokenUserMappingClaim: string;
	    snowflakeUserMappingAttr: string;
	    jwsKeysUrl: string;
	    audienceList: string;
	    anyRoleMode: string;
	    networkPolicy: string;
	    oauthClient: string;
	    oauthClientType: string;
	    oauthRedirectUri: string;
	    oauthIssueRefreshTokens: boolean;
	    oauthRefreshTokenValidity: number;
	    samlIdpMetadataUrl: string;
	    samlIdpEntityId: string;
	    samlIdpSsoUrl: string;
	    samlIdpCert: string;
	    samlAllowedUserDomains: string;
	    samlSignRequest: boolean;
	    samlForceAuthn: boolean;
	    scimClient: string;
	    runAsRole: string;
	    syncPassword: boolean;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new SecurityIntegrationParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.enabled = source["enabled"];
	        this.secType = source["secType"];
	        this.authType = source["authType"];
	        this.awsRoleArn = source["awsRoleArn"];
	        this.oauthGrant = source["oauthGrant"];
	        this.oauthTokenEndpoint = source["oauthTokenEndpoint"];
	        this.oauthClientId = source["oauthClientId"];
	        this.oauthClientSecret = source["oauthClientSecret"];
	        this.oauthScopes = source["oauthScopes"];
	        this.externalOauthType = source["externalOauthType"];
	        this.issuer = source["issuer"];
	        this.tokenUserMappingClaim = source["tokenUserMappingClaim"];
	        this.snowflakeUserMappingAttr = source["snowflakeUserMappingAttr"];
	        this.jwsKeysUrl = source["jwsKeysUrl"];
	        this.audienceList = source["audienceList"];
	        this.anyRoleMode = source["anyRoleMode"];
	        this.networkPolicy = source["networkPolicy"];
	        this.oauthClient = source["oauthClient"];
	        this.oauthClientType = source["oauthClientType"];
	        this.oauthRedirectUri = source["oauthRedirectUri"];
	        this.oauthIssueRefreshTokens = source["oauthIssueRefreshTokens"];
	        this.oauthRefreshTokenValidity = source["oauthRefreshTokenValidity"];
	        this.samlIdpMetadataUrl = source["samlIdpMetadataUrl"];
	        this.samlIdpEntityId = source["samlIdpEntityId"];
	        this.samlIdpSsoUrl = source["samlIdpSsoUrl"];
	        this.samlIdpCert = source["samlIdpCert"];
	        this.samlAllowedUserDomains = source["samlAllowedUserDomains"];
	        this.samlSignRequest = source["samlSignRequest"];
	        this.samlForceAuthn = source["samlForceAuthn"];
	        this.scimClient = source["scimClient"];
	        this.runAsRole = source["runAsRole"];
	        this.syncPassword = source["syncPassword"];
	        this.comment = source["comment"];
	    }
	}
	export class StorageIntegrationParams {
	    name: string;
	    caseSensitive: boolean;
	    enabled: boolean;
	    provider: string;
	    awsRoleArn: string;
	    awsExternalId: string;
	    allowedLocations: string;
	    blockedLocations: string;
	    usePrivatelink: boolean;
	    azureTenantId: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new StorageIntegrationParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.enabled = source["enabled"];
	        this.provider = source["provider"];
	        this.awsRoleArn = source["awsRoleArn"];
	        this.awsExternalId = source["awsExternalId"];
	        this.allowedLocations = source["allowedLocations"];
	        this.blockedLocations = source["blockedLocations"];
	        this.usePrivatelink = source["usePrivatelink"];
	        this.azureTenantId = source["azureTenantId"];
	        this.comment = source["comment"];
	    }
	}

}

export namespace keypair {
	
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

}

export namespace maskingpolicy {
	
	export class MaskingArg {
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new MaskingArg(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	export class MaskingPolicyConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    args: MaskingArg[];
	    returnType: string;
	    body: string;
	    comment: string;
	    exemptOtherPolicies: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MaskingPolicyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.args = this.convertValues(source["args"], MaskingArg);
	        this.returnType = source["returnType"];
	        this.body = source["body"];
	        this.comment = source["comment"];
	        this.exemptOtherPolicies = source["exemptOtherPolicies"];
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

export namespace materializedview {
	
	export class MaterializedViewConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    secure: boolean;
	    ifNotExists: boolean;
	    copyGrants: boolean;
	    comment: string;
	    clusterBy: string;
	    tags: snowflake.TagPair[];
	    query: string;
	
	    static createFrom(source: any = {}) {
	        return new MaterializedViewConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.secure = source["secure"];
	        this.ifNotExists = source["ifNotExists"];
	        this.copyGrants = source["copyGrants"];
	        this.comment = source["comment"];
	        this.clusterBy = source["clusterBy"];
	        this.tags = this.convertValues(source["tags"], snowflake.TagPair);
	        this.query = source["query"];
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

export namespace mcp {
	
	export class ERDesignerColumnOut {
	    name: string;
	    dataType: string;
	    isPK: boolean;
	    notNull: boolean;
	    fkRef?: string;
	
	    static createFrom(source: any = {}) {
	        return new ERDesignerColumnOut(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dataType = source["dataType"];
	        this.isPK = source["isPK"];
	        this.notNull = source["notNull"];
	        this.fkRef = source["fkRef"];
	    }
	}
	export class ERDesignerTableOut {
	    schema: string;
	    name: string;
	    columns: ERDesignerColumnOut[];
	
	    static createFrom(source: any = {}) {
	        return new ERDesignerTableOut(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.columns = this.convertValues(source["columns"], ERDesignerColumnOut);
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
	export class SessionInfo {
	    label: string;
	    port: number;
	    executionMode: string;
	    url: string;
	    connectionLabel: string;
	    pinnedRole?: string;
	    pinnedWarehouse?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.port = source["port"];
	        this.executionMode = source["executionMode"];
	        this.url = source["url"];
	        this.connectionLabel = source["connectionLabel"];
	        this.pinnedRole = source["pinnedRole"];
	        this.pinnedWarehouse = source["pinnedWarehouse"];
	    }
	}

}

export namespace migration {
	
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

}

export namespace networkrule {
	
	export class NetworkRuleConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    type: string;
	    mode: string;
	    valueList: string[];
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkRuleConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.type = source["type"];
	        this.mode = source["mode"];
	        this.valueList = source["valueList"];
	        this.comment = source["comment"];
	    }
	}

}

export namespace objects {
	
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

}

export namespace passwordpolicy {
	
	export class PasswordPolicyConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    minLength?: number;
	    maxLength?: number;
	    minUpperCaseChars?: number;
	    minLowerCaseChars?: number;
	    minNumericChars?: number;
	    minSpecialChars?: number;
	    minAgeDays?: number;
	    maxAgeDays?: number;
	    maxRetries?: number;
	    lockoutTimeMins?: number;
	    history?: number;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new PasswordPolicyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.minLength = source["minLength"];
	        this.maxLength = source["maxLength"];
	        this.minUpperCaseChars = source["minUpperCaseChars"];
	        this.minLowerCaseChars = source["minLowerCaseChars"];
	        this.minNumericChars = source["minNumericChars"];
	        this.minSpecialChars = source["minSpecialChars"];
	        this.minAgeDays = source["minAgeDays"];
	        this.maxAgeDays = source["maxAgeDays"];
	        this.maxRetries = source["maxRetries"];
	        this.lockoutTimeMins = source["lockoutTimeMins"];
	        this.history = source["history"];
	        this.comment = source["comment"];
	    }
	}

}

export namespace pipe {
	
	export class PipeConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    autoIngest: boolean;
	    errorIntegration: string;
	    awsSnsTopic: string;
	    integration: string;
	    comment: string;
	    copyStatement: string;
	
	    static createFrom(source: any = {}) {
	        return new PipeConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.autoIngest = source["autoIngest"];
	        this.errorIntegration = source["errorIntegration"];
	        this.awsSnsTopic = source["awsSnsTopic"];
	        this.integration = source["integration"];
	        this.comment = source["comment"];
	        this.copyStatement = source["copyStatement"];
	    }
	}
	export class RefreshPipeConfig {
	    prefix: string;
	    modifiedAfter: string;
	
	    static createFrom(source: any = {}) {
	        return new RefreshPipeConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefix = source["prefix"];
	        this.modifiedAfter = source["modifiedAfter"];
	    }
	}

}

export namespace procedure {
	
	export class Argument {
	    name: string;
	    dataType: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Argument(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dataType = source["dataType"];
	        this.value = source["value"];
	    }
	}

}

export namespace projectionpolicy {
	
	export class ProjectionPolicyConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    body: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectionPolicyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.body = source["body"];
	        this.comment = source["comment"];
	    }
	}

}

export namespace queryhistory {
	
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

}

export namespace querylog {
	
	export class Entry {
	    id: number;
	    // Go type: time
	    timestamp: any;
	    sql: string;
	    queryID: string;
	    status: string;
	    durationMs: number;
	    error: string;
	    source: string;
	    tabID: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.sql = source["sql"];
	        this.queryID = source["queryID"];
	        this.status = source["status"];
	        this.durationMs = source["durationMs"];
	        this.error = source["error"];
	        this.source = source["source"];
	        this.tabID = source["tabID"];
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

export namespace queryprofile {
	
	export class ExplainData {
	    operation: string;
	    objectName?: string;
	    bytesAssigned?: number;
	    partitionsScanned?: number;
	    partitionsTotal?: number;
	    joinType?: string;
	    estimatedRows?: number;
	
	    static createFrom(source: any = {}) {
	        return new ExplainData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operation = source["operation"];
	        this.objectName = source["objectName"];
	        this.bytesAssigned = source["bytesAssigned"];
	        this.partitionsScanned = source["partitionsScanned"];
	        this.partitionsTotal = source["partitionsTotal"];
	        this.joinType = source["joinType"];
	        this.estimatedRows = source["estimatedRows"];
	    }
	}
	export class ExplainGlobalStats {
	    partitionsTotal: number;
	    partitionsScanned: number;
	    bytesAssigned: number;
	
	    static createFrom(source: any = {}) {
	        return new ExplainGlobalStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.partitionsTotal = source["partitionsTotal"];
	        this.partitionsScanned = source["partitionsScanned"];
	        this.bytesAssigned = source["bytesAssigned"];
	    }
	}
	export class ExplainMarker {
	    startLineNumber: number;
	    startColumn: number;
	    endLineNumber: number;
	    endColumn: number;
	    message: string;
	    severity: number;
	    explainData?: ExplainData;
	
	    static createFrom(source: any = {}) {
	        return new ExplainMarker(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startLineNumber = source["startLineNumber"];
	        this.startColumn = source["startColumn"];
	        this.endLineNumber = source["endLineNumber"];
	        this.endColumn = source["endColumn"];
	        this.message = source["message"];
	        this.severity = source["severity"];
	        this.explainData = this.convertValues(source["explainData"], ExplainData);
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
	export class ExplainNode {
	    id: number;
	    parent?: number;
	    operation: string;
	    objects?: string[];
	    partitionsScanned?: number;
	    partitionsTotal?: number;
	    joinType?: string;
	    estimatedRows?: number;
	
	    static createFrom(source: any = {}) {
	        return new ExplainNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.parent = source["parent"];
	        this.operation = source["operation"];
	        this.objects = source["objects"];
	        this.partitionsScanned = source["partitionsScanned"];
	        this.partitionsTotal = source["partitionsTotal"];
	        this.joinType = source["joinType"];
	        this.estimatedRows = source["estimatedRows"];
	    }
	}
	export class ExplainPlan {
	    GlobalStats: ExplainGlobalStats;
	    Operations: ExplainNode[][];
	    tabularFallback?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExplainPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.GlobalStats = this.convertValues(source["GlobalStats"], ExplainGlobalStats);
	        this.Operations = this.convertValues(source["Operations"], ExplainNode);
	        this.tabularFallback = source["tabularFallback"];
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
	export class ExplainResult {
	    plan?: ExplainPlan;
	    diagnostics: ExplainMarker[];
	
	    static createFrom(source: any = {}) {
	        return new ExplainResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.plan = this.convertValues(source["plan"], ExplainPlan);
	        this.diagnostics = this.convertValues(source["diagnostics"], ExplainMarker);
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
	export class OperatorStat {
	    queryId: string;
	    stepId: number;
	    operatorId: number;
	    parentOperators: number[];
	    operatorType: string;
	    operatorStatistics?: any;
	    executionTimeBreakdown?: any;
	    operatorAttributes?: any;
	
	    static createFrom(source: any = {}) {
	        return new OperatorStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.queryId = source["queryId"];
	        this.stepId = source["stepId"];
	        this.operatorId = source["operatorId"];
	        this.parentOperators = source["parentOperators"];
	        this.operatorType = source["operatorType"];
	        this.operatorStatistics = source["operatorStatistics"];
	        this.executionTimeBreakdown = source["executionTimeBreakdown"];
	        this.operatorAttributes = source["operatorAttributes"];
	    }
	}

}

export namespace rowaccesspolicy {
	
	export class RowAccessArg {
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new RowAccessArg(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	export class RowAccessPolicyConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    args: RowAccessArg[];
	    body: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new RowAccessPolicyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.args = this.convertValues(source["args"], RowAccessArg);
	        this.body = source["body"];
	        this.comment = source["comment"];
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

export namespace secret {
	
	export class SecretConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    type: string;
	    oauthFlow: string;
	    apiAuthentication: string;
	    oauthScopes: string;
	    oauthRefreshToken: string;
	    oauthRefreshTokenExpiry: string;
	    enabled: boolean;
	    username: string;
	    password: string;
	    secretString: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new SecretConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.type = source["type"];
	        this.oauthFlow = source["oauthFlow"];
	        this.apiAuthentication = source["apiAuthentication"];
	        this.oauthScopes = source["oauthScopes"];
	        this.oauthRefreshToken = source["oauthRefreshToken"];
	        this.oauthRefreshTokenExpiry = source["oauthRefreshTokenExpiry"];
	        this.enabled = source["enabled"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.secretString = source["secretString"];
	        this.comment = source["comment"];
	    }
	}

}

export namespace service {
	
	export class TemplateVar {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new TemplateVar(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class ServiceConfig {
	    name: string;
	    caseSensitive: boolean;
	    ifNotExists: boolean;
	    computePool: string;
	    specSource: string;
	    template: boolean;
	    specInline: string;
	    specStage: string;
	    specFile: string;
	    templateVars: TemplateVar[];
	    externalAccessIntegrations: string;
	    autoResume: string;
	    minInstances: string;
	    maxInstances: string;
	    queryWarehouse: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.ifNotExists = source["ifNotExists"];
	        this.computePool = source["computePool"];
	        this.specSource = source["specSource"];
	        this.template = source["template"];
	        this.specInline = source["specInline"];
	        this.specStage = source["specStage"];
	        this.specFile = source["specFile"];
	        this.templateVars = this.convertValues(source["templateVars"], TemplateVar);
	        this.externalAccessIntegrations = source["externalAccessIntegrations"];
	        this.autoResume = source["autoResume"];
	        this.minInstances = source["minInstances"];
	        this.maxInstances = source["maxInstances"];
	        this.queryWarehouse = source["queryWarehouse"];
	        this.comment = source["comment"];
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

export namespace sessionpolicy {
	
	export class SessionPolicyConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    idleTimeoutMins?: number;
	    uiIdleTimeoutMins?: number;
	    maxLifespanMins?: number;
	    uiMaxLifespanMins?: number;
	    allowedSecondaryRoles: string[];
	    blockedSecondaryRoles: string[];
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionPolicyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.idleTimeoutMins = source["idleTimeoutMins"];
	        this.uiIdleTimeoutMins = source["uiIdleTimeoutMins"];
	        this.maxLifespanMins = source["maxLifespanMins"];
	        this.uiMaxLifespanMins = source["uiMaxLifespanMins"];
	        this.allowedSecondaryRoles = source["allowedSecondaryRoles"];
	        this.blockedSecondaryRoles = source["blockedSecondaryRoles"];
	        this.comment = source["comment"];
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
	    token: string;
	    tokenFilePath: string;
	    oauthClientId: string;
	    oauthClientSecret: string;
	    oauthTokenRequestUrl: string;
	    oauthAuthorizationUrl: string;
	    oauthRedirectUri: string;
	    oauthScope: string;
	    enableSingleUseRefreshTokens: boolean;
	    workloadIdentityProvider: string;
	    workloadIdentityEntraResource: string;
	    workloadIdentityImpersonationPath: string;
	    proxyHost: string;
	    proxyPort: number;
	    proxyUser: string;
	    proxyPassword: string;
	    proxyProtocol: string;
	    noProxy: string;
	
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
	        this.token = source["token"];
	        this.tokenFilePath = source["tokenFilePath"];
	        this.oauthClientId = source["oauthClientId"];
	        this.oauthClientSecret = source["oauthClientSecret"];
	        this.oauthTokenRequestUrl = source["oauthTokenRequestUrl"];
	        this.oauthAuthorizationUrl = source["oauthAuthorizationUrl"];
	        this.oauthRedirectUri = source["oauthRedirectUri"];
	        this.oauthScope = source["oauthScope"];
	        this.enableSingleUseRefreshTokens = source["enableSingleUseRefreshTokens"];
	        this.workloadIdentityProvider = source["workloadIdentityProvider"];
	        this.workloadIdentityEntraResource = source["workloadIdentityEntraResource"];
	        this.workloadIdentityImpersonationPath = source["workloadIdentityImpersonationPath"];
	        this.proxyHost = source["proxyHost"];
	        this.proxyPort = source["proxyPort"];
	        this.proxyUser = source["proxyUser"];
	        this.proxyPassword = source["proxyPassword"];
	        this.proxyProtocol = source["proxyProtocol"];
	        this.noProxy = source["noProxy"];
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
	
	export class AccountSecret {
	    name: string;
	    databaseName: string;
	    schemaName: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountSecret(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.databaseName = source["databaseName"];
	        this.schemaName = source["schemaName"];
	    }
	}
	export class ApiIntegration {
	    name: string;
	    type: string;
	    enabled: boolean;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new ApiIntegration(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.enabled = source["enabled"];
	        this.comment = source["comment"];
	    }
	}
	export class ClientVersionInfo {
	    clientId: string;
	    clientAppId: string;
	    minimumSupportedVersion: string;
	    minimumNearingEndOfSupportVersion: string;
	    recommendedVersion: string;
	    deprecatedVersions: string[];
	
	    static createFrom(source: any = {}) {
	        return new ClientVersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.clientAppId = source["clientAppId"];
	        this.minimumSupportedVersion = source["minimumSupportedVersion"];
	        this.minimumNearingEndOfSupportVersion = source["minimumNearingEndOfSupportVersion"];
	        this.recommendedVersion = source["recommendedVersion"];
	        this.deprecatedVersions = source["deprecatedVersions"];
	    }
	}
	export class CollationLocale {
	    code: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new CollationLocale(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	    }
	}
	export class CollationOption {
	    value: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new CollationOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.label = source["label"];
	    }
	}
	export class CollationSpecifier {
	    code: string;
	    name: string;
	    category: string;
	
	    static createFrom(source: any = {}) {
	        return new CollationSpecifier(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.category = source["category"];
	    }
	}
	export class ColumnInfo {
	    name: string;
	    dataType: string;
	    nullable: boolean;
	    isPrimaryKey: boolean;
	    isUnique: boolean;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new ColumnInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dataType = source["dataType"];
	        this.nullable = source["nullable"];
	        this.isPrimaryKey = source["isPrimaryKey"];
	        this.isUnique = source["isUnique"];
	        this.comment = source["comment"];
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
	    token: string;
	    tokenFilePath: string;
	    oauthClientId: string;
	    oauthClientSecret: string;
	    oauthTokenRequestUrl: string;
	    oauthAuthorizationUrl: string;
	    oauthRedirectUri: string;
	    oauthScope: string;
	    enableSingleUseRefreshTokens: boolean;
	    workloadIdentityProvider: string;
	    workloadIdentityEntraResource: string;
	    workloadIdentityImpersonationPath: string;
	    proxyHost: string;
	    proxyPort: number;
	    proxyUser: string;
	    proxyPassword: string;
	    proxyProtocol: string;
	    noProxy: string;
	
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
	        this.token = source["token"];
	        this.tokenFilePath = source["tokenFilePath"];
	        this.oauthClientId = source["oauthClientId"];
	        this.oauthClientSecret = source["oauthClientSecret"];
	        this.oauthTokenRequestUrl = source["oauthTokenRequestUrl"];
	        this.oauthAuthorizationUrl = source["oauthAuthorizationUrl"];
	        this.oauthRedirectUri = source["oauthRedirectUri"];
	        this.oauthScope = source["oauthScope"];
	        this.enableSingleUseRefreshTokens = source["enableSingleUseRefreshTokens"];
	        this.workloadIdentityProvider = source["workloadIdentityProvider"];
	        this.workloadIdentityEntraResource = source["workloadIdentityEntraResource"];
	        this.workloadIdentityImpersonationPath = source["workloadIdentityImpersonationPath"];
	        this.proxyHost = source["proxyHost"];
	        this.proxyPort = source["proxyPort"];
	        this.proxyUser = source["proxyUser"];
	        this.proxyPassword = source["proxyPassword"];
	        this.proxyProtocol = source["proxyProtocol"];
	        this.noProxy = source["noProxy"];
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
	export class DbtProjectVersion {
	    version: string;
	    alias: string;
	    isDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DbtProjectVersion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.alias = source["alias"];
	        this.isDefault = source["isDefault"];
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
	export class GitBranch {
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new GitBranch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	    }
	}
	export class GitRepoEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	    size?: number;
	
	    static createFrom(source: any = {}) {
	        return new GitRepoEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.size = source["size"];
	    }
	}
	export class GitTag {
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new GitTag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
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
	    namedFormat: string;
	
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
	        this.namedFormat = source["namedFormat"];
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
	export class SecurityIntegration {
	    name: string;
	    type: string;
	    category: string;
	    enabled: boolean;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new SecurityIntegration(source);
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
	export class StageSummary {
	    name: string;
	    type: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new StageSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.url = source["url"];
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
	export class TagPair {
	    name: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new TagPair(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	    }
	}
	export class UserFunction {
	    name: string;
	    schema: string;
	    database: string;
	    qualified: string;
	    arguments: string;
	
	    static createFrom(source: any = {}) {
	        return new UserFunction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.schema = source["schema"];
	        this.database = source["database"];
	        this.qualified = source["qualified"];
	        this.arguments = source["arguments"];
	    }
	}
	export class WorkspaceInfo {
	    name: string;
	    database: string;
	    schema: string;
	    owner: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.owner = source["owner"];
	    }
	}

}

export namespace snowgitrepo {
	
	export class GitRepositoryConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    originUrl: string;
	    apiIntegration: string;
	    gitCredentials: string;
	    comment: string;
	    tags: snowflake.TagPair[];
	
	    static createFrom(source: any = {}) {
	        return new GitRepositoryConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.originUrl = source["originUrl"];
	        this.apiIntegration = source["apiIntegration"];
	        this.gitCredentials = source["gitCredentials"];
	        this.comment = source["comment"];
	        this.tags = this.convertValues(source["tags"], snowflake.TagPair);
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

export namespace snowpark {
	
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
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NotebookSqlResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.rowCount = source["rowCount"];
	        this.queryID = source["queryID"];
	        this.truncated = source["truncated"];
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

}

export namespace sqleditor {
	
	export class UsingClauseInfo {
	    inUsing: boolean;
	    isPartial: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UsingClauseInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inUsing = source["inUsing"];
	        this.isPartial = source["isPartial"];
	    }
	}
	export class InEditorTableDef {
	    db: string;
	    schema: string;
	    name: string;
	    cols: ColInfo[];
	
	    static createFrom(source: any = {}) {
	        return new InEditorTableDef(source);
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
	export class UseContext {
	    database: string;
	    schema: string;
	
	    static createFrom(source: any = {}) {
	        return new UseContext(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	    }
	}
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
	export class CTEColumnEntry {
	    name: string;
	    cols: ColInfo[];
	
	    static createFrom(source: any = {}) {
	        return new CTEColumnEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
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
	export class AutocompleteContext {
	    statementRanges: StatementRange[];
	    currentStmt: string;
	    currentStmtIdx: number;
	    scripting: ScriptingCompletionResult;
	    tableRefs: JoinTableRef[];
	    cteColumns: CTEColumnEntry[];
	    useContext?: UseContext;
	    resolvedRefs?: ResolvedRef[];
	    inEditorTables?: InEditorTableDef[];
	    isDatatypeContext: boolean;
	    isInJoinOnClause: boolean;
	    usingClause?: UsingClauseInfo;
	
	    static createFrom(source: any = {}) {
	        return new AutocompleteContext(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statementRanges = this.convertValues(source["statementRanges"], StatementRange);
	        this.currentStmt = source["currentStmt"];
	        this.currentStmtIdx = source["currentStmtIdx"];
	        this.scripting = this.convertValues(source["scripting"], ScriptingCompletionResult);
	        this.tableRefs = this.convertValues(source["tableRefs"], JoinTableRef);
	        this.cteColumns = this.convertValues(source["cteColumns"], CTEColumnEntry);
	        this.useContext = this.convertValues(source["useContext"], UseContext);
	        this.resolvedRefs = this.convertValues(source["resolvedRefs"], ResolvedRef);
	        this.inEditorTables = this.convertValues(source["inEditorTables"], InEditorTableDef);
	        this.isDatatypeContext = source["isDatatypeContext"];
	        this.isInJoinOnClause = source["isInJoinOnClause"];
	        this.usingClause = this.convertValues(source["usingClause"], UsingClauseInfo);
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
	export class SessionContext {
	    database: string;
	    schema: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionContext(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	    }
	}
	export class StoreObject {
	    db: string;
	    schema: string;
	    name: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new StoreObject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.db = source["db"];
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	    }
	}
	export class AutocompleteContextRequest {
	    sql: string;
	    cursorOffset: number;
	    storeObjects: StoreObject[];
	    session?: SessionContext;
	    lineUpToWord: string;
	
	    static createFrom(source: any = {}) {
	        return new AutocompleteContextRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sql = source["sql"];
	        this.cursorOffset = source["cursorOffset"];
	        this.storeObjects = this.convertValues(source["storeObjects"], StoreObject);
	        this.session = this.convertValues(source["session"], SessionContext);
	        this.lineUpToWord = source["lineUpToWord"];
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
	    code?: string;
	
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
	        this.code = source["code"];
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
	
	export class LineDiff {
	    added: number[];
	    modified: number[];
	    deleted: number[];
	
	    static createFrom(source: any = {}) {
	        return new LineDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.modified = source["modified"];
	        this.deleted = source["deleted"];
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
	    allKnownTables: ResolvedRef[];
	
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
	        this.allKnownTables = this.convertValues(source["allKnownTables"], ResolvedRef);
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

export namespace stage {
	
	export class AlterStageConfig {
	    name: string;
	    database: string;
	    schema: string;
	    action: string;
	    newName: string;
	    caseSensitive: boolean;
	    comment?: string;
	    url?: string;
	    storageIntegration?: string;
	    directoryEnabled?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AlterStageConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.action = source["action"];
	        this.newName = source["newName"];
	        this.caseSensitive = source["caseSensitive"];
	        this.comment = source["comment"];
	        this.url = source["url"];
	        this.storageIntegration = source["storageIntegration"];
	        this.directoryEnabled = source["directoryEnabled"];
	    }
	}
	export class StageConfig {
	    name: string;
	    database: string;
	    schema: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    type: string;
	    url: string;
	    storageIntegration: string;
	    usePrivatelinkEndpoint: boolean;
	    encryptionType: string;
	    kmsKeyId: string;
	    directoryEnabled: boolean;
	    directoryAutoRefresh: boolean;
	    directoryRefreshOnCreate: boolean;
	    directoryNotificationIntegration: string;
	    fileFormatName: string;
	    fileFormat: fileformat.FileFormatConfig;
	    comment: string;
	    tags: string;
	
	    static createFrom(source: any = {}) {
	        return new StageConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.storageIntegration = source["storageIntegration"];
	        this.usePrivatelinkEndpoint = source["usePrivatelinkEndpoint"];
	        this.encryptionType = source["encryptionType"];
	        this.kmsKeyId = source["kmsKeyId"];
	        this.directoryEnabled = source["directoryEnabled"];
	        this.directoryAutoRefresh = source["directoryAutoRefresh"];
	        this.directoryRefreshOnCreate = source["directoryRefreshOnCreate"];
	        this.directoryNotificationIntegration = source["directoryNotificationIntegration"];
	        this.fileFormatName = source["fileFormatName"];
	        this.fileFormat = this.convertValues(source["fileFormat"], fileformat.FileFormatConfig);
	        this.comment = source["comment"];
	        this.tags = source["tags"];
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
	export class StageFile {
	    name: string;
	    size: number;
	    md5: string;
	    lastModified: string;
	
	    static createFrom(source: any = {}) {
	        return new StageFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.size = source["size"];
	        this.md5 = source["md5"];
	        this.lastModified = source["lastModified"];
	    }
	}

}

export namespace streamlit {
	
	export class StreamlitConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    stageLocation: string;
	    mainFile: string;
	    queryWarehouse: string;
	    externalAccessIntegrations: string;
	    title: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new StreamlitConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.stageLocation = source["stageLocation"];
	        this.mainFile = source["mainFile"];
	        this.queryWarehouse = source["queryWarehouse"];
	        this.externalAccessIntegrations = source["externalAccessIntegrations"];
	        this.title = source["title"];
	        this.comment = source["comment"];
	    }
	}

}

export namespace table {
	
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

}

export namespace tag {
	
	export class TagConfig {
	    name: string;
	    caseSensitive: boolean;
	    orReplace: boolean;
	    ifNotExists: boolean;
	    allowedValues: string[];
	    propagate: string;
	    onConflict: string;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new TagConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.caseSensitive = source["caseSensitive"];
	        this.orReplace = source["orReplace"];
	        this.ifNotExists = source["ifNotExists"];
	        this.allowedValues = source["allowedValues"];
	        this.propagate = source["propagate"];
	        this.onConflict = source["onConflict"];
	        this.comment = source["comment"];
	    }
	}

}

export namespace tasks {
	
	export class ExportGraphDDLResult {
	    ddl: string;
	    taskCount: number;
	    failedTasks: string[];
	
	    static createFrom(source: any = {}) {
	        return new ExportGraphDDLResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ddl = source["ddl"];
	        this.taskCount = source["taskCount"];
	        this.failedTasks = source["failedTasks"];
	    }
	}
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
	export class TaskHistoryRow {
	    name: string;
	    state: string;
	    returnValue: string;
	    scheduledTime: string;
	    startTime: string;
	    endTime: string;
	    errorCode: string;
	    errorMessage: string;
	    runId: string;
	    rootTaskId: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskHistoryRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.state = source["state"];
	        this.returnValue = source["returnValue"];
	        this.scheduledTime = source["scheduledTime"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.errorCode = source["errorCode"];
	        this.errorMessage = source["errorMessage"];
	        this.runId = source["runId"];
	        this.rootTaskId = source["rootTaskId"];
	    }
	}
	export class TopologicalOrder {
	    topoOrder: string[];
	    finalizerNames: string[];
	    suspendOrder: string[];
	    resumeOrder: string[];
	
	    static createFrom(source: any = {}) {
	        return new TopologicalOrder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.topoOrder = source["topoOrder"];
	        this.finalizerNames = source["finalizerNames"];
	        this.suspendOrder = source["suspendOrder"];
	        this.resumeOrder = source["resumeOrder"];
	    }
	}

}

export namespace warehouse {
	
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

