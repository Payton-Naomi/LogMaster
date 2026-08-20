export namespace main {
	
	export class AppSettingsDTO {
	    defaultLogDirectory: string;
	    defaultSaveEnabled: boolean;
	    defaultUploadEnabled: boolean;
	    segmentMaxAgeSeconds: number;
	    segmentMaxBytes: number;
	    noLogTimeoutSeconds: number;
	    maxLogLines: number;
	    logFontSize: number;
	    autoWrap: boolean;
	    maxDiskBytes: number;
	    storageWarningPercent: number;
	    autoDeleteUploaded: boolean;
	    uploadedRetentionHours: number;
	    backendUrl: string;
	    uploadIntervalSeconds: number;
	    uploadConcurrency: number;
	    uploadGzip: boolean;
	    programName: string;
	    programVersion: string;
	    buildVersion: string;
	    updateDate: string;
	    companyName: string;
	    communityTitle: string;
	    communityText: string;
	    communityUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new AppSettingsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultLogDirectory = source["defaultLogDirectory"];
	        this.defaultSaveEnabled = source["defaultSaveEnabled"];
	        this.defaultUploadEnabled = source["defaultUploadEnabled"];
	        this.segmentMaxAgeSeconds = source["segmentMaxAgeSeconds"];
	        this.segmentMaxBytes = source["segmentMaxBytes"];
	        this.noLogTimeoutSeconds = source["noLogTimeoutSeconds"];
	        this.maxLogLines = source["maxLogLines"];
	        this.logFontSize = source["logFontSize"];
	        this.autoWrap = source["autoWrap"];
	        this.maxDiskBytes = source["maxDiskBytes"];
	        this.storageWarningPercent = source["storageWarningPercent"];
	        this.autoDeleteUploaded = source["autoDeleteUploaded"];
	        this.uploadedRetentionHours = source["uploadedRetentionHours"];
	        this.backendUrl = source["backendUrl"];
	        this.uploadIntervalSeconds = source["uploadIntervalSeconds"];
	        this.uploadConcurrency = source["uploadConcurrency"];
	        this.uploadGzip = source["uploadGzip"];
	        this.programName = source["programName"];
	        this.programVersion = source["programVersion"];
	        this.buildVersion = source["buildVersion"];
	        this.updateDate = source["updateDate"];
	        this.companyName = source["companyName"];
	        this.communityTitle = source["communityTitle"];
	        this.communityText = source["communityText"];
	        this.communityUrl = source["communityUrl"];
	    }
	}
	export class CatalogChangeDTO {
	    kind: string;
	    entity: string;
	    id: string;
	    name: string;
	    oldValue: string;
	    newValue: string;
	    impact: string;
	
	    static createFrom(source: any = {}) {
	        return new CatalogChangeDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.entity = source["entity"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.oldValue = source["oldValue"];
	        this.newValue = source["newValue"];
	        this.impact = source["impact"];
	    }
	}
	export class CatalogKeywordGroup {
	    id: string;
	    name: string;
	    scope: string;
	    rules: CatalogKeywordRule[];
	
	    static createFrom(source: any = {}) {
	        return new CatalogKeywordGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.scope = source["scope"];
	        this.rules = this.convertValues(source["rules"], CatalogKeywordRule);
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
	export class CatalogKeywordRule {
	    id: string;
	    name: string;
	    match: string;
	    mode: string;
	    caseSensitive: boolean;
	    level?: string;
	    description?: string;
	    readOnly?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CatalogKeywordRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.match = source["match"];
	        this.mode = source["mode"];
	        this.caseSensitive = source["caseSensitive"];
	        this.level = source["level"];
	        this.description = source["description"];
	        this.readOnly = source["readOnly"];
	    }
	}
	export class CatalogKeywordProfile {
	    id: string;
	    name: string;
	    rules: CatalogKeywordRule[];
	    groups: CatalogKeywordGroup[];
	
	    static createFrom(source: any = {}) {
	        return new CatalogKeywordProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.rules = this.convertValues(source["rules"], CatalogKeywordRule);
	        this.groups = this.convertValues(source["groups"], CatalogKeywordGroup);
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
	export class CatalogTask {
	    id: string;
	    name: string;
	    type: string;
	    keywordProfiles: CatalogKeywordProfile[];
	
	    static createFrom(source: any = {}) {
	        return new CatalogTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.keywordProfiles = this.convertValues(source["keywordProfiles"], CatalogKeywordProfile);
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
	export class CatalogProject {
	    id: string;
	    name: string;
	    versions: string[];
	    tasks: CatalogTask[];
	
	    static createFrom(source: any = {}) {
	        return new CatalogProject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.versions = source["versions"];
	        this.tasks = this.convertValues(source["tasks"], CatalogTask);
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
	export class CatalogConfig {
	    schemaVersion: number;
	    projects: CatalogProject[];
	
	    static createFrom(source: any = {}) {
	        return new CatalogConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.projects = this.convertValues(source["projects"], CatalogProject);
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
	export class CatalogFileDTO {
	    name: string;
	    path: string;
	    exists: boolean;
	    sizeBytes: number;
	    // Go type: time
	    modifiedAt?: any;
	    editable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CatalogFileDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.sizeBytes = source["sizeBytes"];
	        this.modifiedAt = this.convertValues(source["modifiedAt"], null);
	        this.editable = source["editable"];
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
	export class CatalogFilesDTO {
	    directory: string;
	    files: CatalogFileDTO[];
	    cloudCache: CatalogFileDTO;
	
	    static createFrom(source: any = {}) {
	        return new CatalogFilesDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.files = this.convertValues(source["files"], CatalogFileDTO);
	        this.cloudCache = this.convertValues(source["cloudCache"], CatalogFileDTO);
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
	export class CatalogImportPreviewDTO {
	    token: string;
	    fileName: string;
	    sha256: string;
	    changes: CatalogChangeDTO[];
	    warnings: string[];
	    added: number;
	    modified: number;
	    deleted: number;
	    unchanged: number;
	
	    static createFrom(source: any = {}) {
	        return new CatalogImportPreviewDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.fileName = source["fileName"];
	        this.sha256 = source["sha256"];
	        this.changes = this.convertValues(source["changes"], CatalogChangeDTO);
	        this.warnings = source["warnings"];
	        this.added = source["added"];
	        this.modified = source["modified"];
	        this.deleted = source["deleted"];
	        this.unchanged = source["unchanged"];
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
	
	
	
	
	
	export class CloudKeywordSyncResult {
	    count: number;
	    // Go type: time
	    syncedAt: any;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudKeywordSyncResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.syncedAt = this.convertValues(source["syncedAt"], null);
	        this.message = source["message"];
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
	export class DeviceConfigDTO {
	    deviceId: string;
	    name: string;
	    portName: string;
	    baudRate: number;
	    dataBits: number;
	    stopBits: number;
	    parity: string;
	    handshake: string;
	    encoding: string;
	    dtr: boolean;
	    rts: boolean;
	    readTimeoutMs: number;
	    writeTimeoutMs: number;
	    idleGapMs: number;
	    maxFrameBytes: number;
	    configured: boolean;
	    projectId: string;
	    projectName: string;
	    version: string;
	    testTaskId: string;
	    testTaskName: string;
	    uploaderName: string;
	    uploaderEmail: string;
	    remark: string;
	    scenarioIds: string[];
	    keywordProfileId: string;
	    keywordRuleIds: string[];
	    keywordMatchingEnabled: boolean;
	    saveEnabled: boolean;
	    uploadEnabled: boolean;
	    noLogTimeoutSeconds: number;
	    vid: string;
	    pid: string;
	    usbSerial: string;
	    location: string;
	    uploadSessionId?: string;
	    queryCode?: string;
	    uploadSetupId?: string;
	    uploadSetupState?: string;
	    uploadConfigFingerprint?: string;
	    configSnapshot?: string;
	    previousConfigAvailable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeviceConfigDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceId = source["deviceId"];
	        this.name = source["name"];
	        this.portName = source["portName"];
	        this.baudRate = source["baudRate"];
	        this.dataBits = source["dataBits"];
	        this.stopBits = source["stopBits"];
	        this.parity = source["parity"];
	        this.handshake = source["handshake"];
	        this.encoding = source["encoding"];
	        this.dtr = source["dtr"];
	        this.rts = source["rts"];
	        this.readTimeoutMs = source["readTimeoutMs"];
	        this.writeTimeoutMs = source["writeTimeoutMs"];
	        this.idleGapMs = source["idleGapMs"];
	        this.maxFrameBytes = source["maxFrameBytes"];
	        this.configured = source["configured"];
	        this.projectId = source["projectId"];
	        this.projectName = source["projectName"];
	        this.version = source["version"];
	        this.testTaskId = source["testTaskId"];
	        this.testTaskName = source["testTaskName"];
	        this.uploaderName = source["uploaderName"];
	        this.uploaderEmail = source["uploaderEmail"];
	        this.remark = source["remark"];
	        this.scenarioIds = source["scenarioIds"];
	        this.keywordProfileId = source["keywordProfileId"];
	        this.keywordRuleIds = source["keywordRuleIds"];
	        this.keywordMatchingEnabled = source["keywordMatchingEnabled"];
	        this.saveEnabled = source["saveEnabled"];
	        this.uploadEnabled = source["uploadEnabled"];
	        this.noLogTimeoutSeconds = source["noLogTimeoutSeconds"];
	        this.vid = source["vid"];
	        this.pid = source["pid"];
	        this.usbSerial = source["usbSerial"];
	        this.location = source["location"];
	        this.uploadSessionId = source["uploadSessionId"];
	        this.queryCode = source["queryCode"];
	        this.uploadSetupId = source["uploadSetupId"];
	        this.uploadSetupState = source["uploadSetupState"];
	        this.uploadConfigFingerprint = source["uploadConfigFingerprint"];
	        this.configSnapshot = source["configSnapshot"];
	        this.previousConfigAvailable = source["previousConfigAvailable"];
	    }
	}
	export class DeviceConfigSaveResult {
	    saved: boolean;
	    uploadReady: boolean;
	    queryCode?: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceConfigSaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.saved = source["saved"];
	        this.uploadReady = source["uploadReady"];
	        this.queryCode = source["queryCode"];
	        this.message = source["message"];
	    }
	}
	export class DeviceStateDTO {
	    deviceId: string;
	    name: string;
	    portName: string;
	    status: string;
	    lastError?: string;
	    droppedEvents: number;
	    linesReceived: number;
	    reconnects: number;
	    ruleCounts: Record<string, number>;
	    config: DeviceConfigDTO;
	    enabled: boolean;
	    detected: boolean;
	    persisting: boolean;
	    uploadEnabled: boolean;
	    noLogAlert: boolean;
	    // Go type: time
	    lastLogAt?: any;
	    sessionId: string;
	    configStatus: string;
	    storageBytes: number;
	    pendingUploads: number;
	
	    static createFrom(source: any = {}) {
	        return new DeviceStateDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceId = source["deviceId"];
	        this.name = source["name"];
	        this.portName = source["portName"];
	        this.status = source["status"];
	        this.lastError = source["lastError"];
	        this.droppedEvents = source["droppedEvents"];
	        this.linesReceived = source["linesReceived"];
	        this.reconnects = source["reconnects"];
	        this.ruleCounts = source["ruleCounts"];
	        this.config = this.convertValues(source["config"], DeviceConfigDTO);
	        this.enabled = source["enabled"];
	        this.detected = source["detected"];
	        this.persisting = source["persisting"];
	        this.uploadEnabled = source["uploadEnabled"];
	        this.noLogAlert = source["noLogAlert"];
	        this.lastLogAt = this.convertValues(source["lastLogAt"], null);
	        this.sessionId = source["sessionId"];
	        this.configStatus = source["configStatus"];
	        this.storageBytes = source["storageBytes"];
	        this.pendingUploads = source["pendingUploads"];
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
	export class DeviceStorageDTO {
	    deviceId: string;
	    totalBytes: number;
	    fileCount: number;
	    pendingBytes: number;
	    uploadedBytes: number;
	    // Go type: time
	    earliestAt?: any;
	    // Go type: time
	    latestAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new DeviceStorageDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceId = source["deviceId"];
	        this.totalBytes = source["totalBytes"];
	        this.fileCount = source["fileCount"];
	        this.pendingBytes = source["pendingBytes"];
	        this.uploadedBytes = source["uploadedBytes"];
	        this.earliestAt = this.convertValues(source["earliestAt"], null);
	        this.latestAt = this.convertValues(source["latestAt"], null);
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
	export class HistoryFileDTO {
	    id: string;
	    sessionId: string;
	    path: string;
	    fileName: string;
	    deviceId: string;
	    portName: string;
	    projectId: string;
	    projectName: string;
	    version: string;
	    testTaskId: string;
	    testTaskName: string;
	    firstSequence: number;
	    lastSequence: number;
	    lineCount: number;
	    sizeBytes: number;
	    sha256: string;
	    uploadState: string;
	    queryCode: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    completedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new HistoryFileDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.path = source["path"];
	        this.fileName = source["fileName"];
	        this.deviceId = source["deviceId"];
	        this.portName = source["portName"];
	        this.projectId = source["projectId"];
	        this.projectName = source["projectName"];
	        this.version = source["version"];
	        this.testTaskId = source["testTaskId"];
	        this.testTaskName = source["testTaskName"];
	        this.firstSequence = source["firstSequence"];
	        this.lastSequence = source["lastSequence"];
	        this.lineCount = source["lineCount"];
	        this.sizeBytes = source["sizeBytes"];
	        this.sha256 = source["sha256"];
	        this.uploadState = source["uploadState"];
	        this.queryCode = source["queryCode"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.completedAt = this.convertValues(source["completedAt"], null);
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
	export class HistoryPageDTO {
	    items: HistoryFileDTO[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryPageDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], HistoryFileDTO);
	        this.total = source["total"];
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
	export class HistoryPreviewDTO {
	    file: HistoryFileDTO;
	    lines: string[];
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HistoryPreviewDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = this.convertValues(source["file"], HistoryFileDTO);
	        this.lines = source["lines"];
	        this.truncated = source["truncated"];
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
	export class HistoryQueryDTO {
	    deviceId: string;
	    projectId: string;
	    version: string;
	    testTaskId: string;
	    search: string;
	    state: string;
	    from: string;
	    to: string;
	    offset: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryQueryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceId = source["deviceId"];
	        this.projectId = source["projectId"];
	        this.version = source["version"];
	        this.testTaskId = source["testTaskId"];
	        this.search = source["search"];
	        this.state = source["state"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	    }
	}
	export class KeywordHitDTO {
	    id: string;
	    sessionId: string;
	    deviceId: string;
	    ruleId: string;
	    ruleName: string;
	    // Go type: time
	    matchedAt: any;
	    sequence: number;
	    lineText: string;
	
	    static createFrom(source: any = {}) {
	        return new KeywordHitDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.deviceId = source["deviceId"];
	        this.ruleId = source["ruleId"];
	        this.ruleName = source["ruleName"];
	        this.matchedAt = this.convertValues(source["matchedAt"], null);
	        this.sequence = source["sequence"];
	        this.lineText = source["lineText"];
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
	export class PortInfo {
	    name: string;
	    vid: string;
	    pid: string;
	    usbSerial: string;
	    location: string;
	    manufacturer: string;
	    product: string;
	    isUSB: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PortInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.vid = source["vid"];
	        this.pid = source["pid"];
	        this.usbSerial = source["usbSerial"];
	        this.location = source["location"];
	        this.manufacturer = source["manufacturer"];
	        this.product = source["product"];
	        this.isUSB = source["isUSB"];
	    }
	}
	export class QueueStatus {
	    pending: number;
	    uploading: number;
	    uploaded: number;
	    uncertain: number;
	    dead: number;
	    diskUsagePercent: number;
	    diskUsageText: string;
	
	    static createFrom(source: any = {}) {
	        return new QueueStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pending = source["pending"];
	        this.uploading = source["uploading"];
	        this.uploaded = source["uploaded"];
	        this.uncertain = source["uncertain"];
	        this.dead = source["dead"];
	        this.diskUsagePercent = source["diskUsagePercent"];
	        this.diskUsageText = source["diskUsageText"];
	    }
	}
	export class UncertainCheckDTO {
	    batchId: string;
	    queryCode: string;
	    status: string;
	    uploadId?: string;
	    taskId?: string;
	    matched: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UncertainCheckDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.batchId = source["batchId"];
	        this.queryCode = source["queryCode"];
	        this.status = source["status"];
	        this.uploadId = source["uploadId"];
	        this.taskId = source["taskId"];
	        this.matched = source["matched"];
	    }
	}
	export class UploadBatchDTO {
	    id: string;
	    state: string;
	    deviceId: string;
	    fileName: string;
	    sizeBytes: number;
	    sha256: string;
	    attemptCount: number;
	    lastError: string;
	    // Go type: time
	    createdAt: any;
	    projectName: string;
	    version: string;
	    sessionId: string;
	    queryCode: string;
	    uploadPosition: number;
	    bytesTotal: number;
	    bytesSent: number;
	    speedBytes: number;
	    // Go type: time
	    startedAt?: any;
	    // Go type: time
	    completedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new UploadBatchDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.state = source["state"];
	        this.deviceId = source["deviceId"];
	        this.fileName = source["fileName"];
	        this.sizeBytes = source["sizeBytes"];
	        this.sha256 = source["sha256"];
	        this.attemptCount = source["attemptCount"];
	        this.lastError = source["lastError"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.projectName = source["projectName"];
	        this.version = source["version"];
	        this.sessionId = source["sessionId"];
	        this.queryCode = source["queryCode"];
	        this.uploadPosition = source["uploadPosition"];
	        this.bytesTotal = source["bytesTotal"];
	        this.bytesSent = source["bytesSent"];
	        this.speedBytes = source["speedBytes"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.completedAt = this.convertValues(source["completedAt"], null);
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
	export class UploadQueuePageDTO {
	    items: UploadBatchDTO[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new UploadQueuePageDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], UploadBatchDTO);
	        this.total = source["total"];
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
	export class UploadQueueQueryDTO {
	    deviceId: string;
	    states: string[];
	    search: string;
	    includeUploaded: boolean;
	    offset: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new UploadQueueQueryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceId = source["deviceId"];
	        this.states = source["states"];
	        this.search = source["search"];
	        this.includeUploaded = source["includeUploaded"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	    }
	}

}

