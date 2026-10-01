export namespace main {

	export class MCPServer {
	    id: string;
	    name: string;
	    target: string;
	    scope: string;
	    path: string;
	    transport?: string;
	    endpoint?: string;
	    command?: string;
	    enabled?: boolean;

	    static createFrom(source: any = {}) {
	        return new MCPServer(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.target = source["target"];
	        this.scope = source["scope"];
	        this.path = source["path"];
	        this.transport = source["transport"];
	        this.endpoint = source["endpoint"];
	        this.command = source["command"];
	        this.enabled = source["enabled"];
	    }
	}
	export class MemoryEntry {
	    id: string;
	    name: string;
	    scope: string;
	    path: string;
	    bytes: number;
	    modifiedAt?: string;

	    static createFrom(source: any = {}) {
	        return new MemoryEntry(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.scope = source["scope"];
	        this.path = source["path"];
	        this.bytes = source["bytes"];
	        this.modifiedAt = source["modifiedAt"];
	    }
	}
	export class RuntimeDocument {
	    id: string;
	    target: string;
	    scope: string;
	    kind: string;
	    format: string;
	    path: string;
	    exists: boolean;
	    bytes: number;
	    modifiedAt?: string;

	    static createFrom(source: any = {}) {
	        return new RuntimeDocument(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.target = source["target"];
	        this.scope = source["scope"];
	        this.kind = source["kind"];
	        this.format = source["format"];
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.bytes = source["bytes"];
	        this.modifiedAt = source["modifiedAt"];
	    }
	}
	export class AdvancedState {
	    target: string;
	    codexHome: string;
	    piHome: string;
	    memoryRoot: string;
	    documents: RuntimeDocument[];
	    memories: MemoryEntry[];
	    mcpServers: MCPServer[];
	    diagnostics: string[];

	    static createFrom(source: any = {}) {
	        return new AdvancedState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.codexHome = source["codexHome"];
	        this.piHome = source["piHome"];
	        this.memoryRoot = source["memoryRoot"];
	        this.documents = this.convertValues(source["documents"], RuntimeDocument);
	        this.memories = this.convertValues(source["memories"], MemoryEntry);
	        this.mcpServers = this.convertValues(source["mcpServers"], MCPServer);
	        this.diagnostics = source["diagnostics"];
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
	export class SkillInfo {
	    name: string;
	    title: string;
	    description: string;
	    path: string;
	    system: boolean;
	    hasScripts: boolean;
	    fileCount: number;
	    sizeBytes: number;
	    modifiedAt?: string;
	    market?: string;
	    repository?: string;
	    version?: string;
	    author?: string;
	    license?: string;
	    url?: string;
	    trashId?: string;
	    sourceId?: string;

	    static createFrom(source: any = {}) {
	        return new SkillInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.path = source["path"];
	        this.system = source["system"];
	        this.hasScripts = source["hasScripts"];
	        this.fileCount = source["fileCount"];
	        this.sizeBytes = source["sizeBytes"];
	        this.modifiedAt = source["modifiedAt"];
	        this.market = source["market"];
	        this.repository = source["repository"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.license = source["license"];
	        this.url = source["url"];
	        this.trashId = source["trashId"];
	        this.sourceId = source["sourceId"];
	    }
	}
	export class AgentDocument {
	    id: string;
	    target: string;
	    scope: string;
	    kind: string;
	    path: string;
	    exists: boolean;
	    loaded: boolean;
	    overridden: boolean;
	    bytes: number;
	    modifiedAt?: string;

	    static createFrom(source: any = {}) {
	        return new AgentDocument(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.target = source["target"];
	        this.scope = source["scope"];
	        this.kind = source["kind"];
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.loaded = source["loaded"];
	        this.overridden = source["overridden"];
	        this.bytes = source["bytes"];
	        this.modifiedAt = source["modifiedAt"];
	    }
	}
	export class Workspace {
	    id: string;
	    name: string;
	    path: string;
	    lastUsed?: string;

	    static createFrom(source: any = {}) {
	        return new Workspace(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.lastUsed = source["lastUsed"];
	    }
	}
	export class AgentConfigState {
	    workspace: Workspace;
	    documents: AgentDocument[];
	    codexHome: string;
	    piHome: string;
	    sharedSkillsRoot: string;
	    sharedSkills: SkillInfo[];

	    static createFrom(source: any = {}) {
	        return new AgentConfigState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = this.convertValues(source["workspace"], Workspace);
	        this.documents = this.convertValues(source["documents"], AgentDocument);
	        this.codexHome = source["codexHome"];
	        this.piHome = source["piHome"];
	        this.sharedSkillsRoot = source["sharedSkillsRoot"];
	        this.sharedSkills = this.convertValues(source["sharedSkills"], SkillInfo);
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

	export class Model {
	    id: string;
	    owned_by?: string;
	    supportsImages?: boolean;

	    static createFrom(source: any = {}) {
	        return new Model(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.owned_by = source["owned_by"];
	        this.supportsImages = source["supportsImages"];
	    }
	}
	export class ProfileView {
	    id: string;
	    name: string;
	    baseUrl: string;
	    hasApiKey: boolean;
	    selectedModel: string;
	    models: Model[];

	    static createFrom(source: any = {}) {
	        return new ProfileView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.hasApiKey = source["hasApiKey"];
	        this.selectedModel = source["selectedModel"];
	        this.models = this.convertValues(source["models"], Model);
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
	export class AppState {
	    target: string;
	    version: string;
	    profiles: ProfileView[];
	    activeProfileId: string;
	    activeProvider: string;
	    activeModel: string;
	    configPath: string;
	    chatGptRunning: boolean;
	    canRestore: boolean;
	    chatGptTarget: string;
	    chatGptResolvedTarget: string;
	    chatGptTargetError: string;

	    static createFrom(source: any = {}) {
	        return new AppState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.version = source["version"];
	        this.profiles = this.convertValues(source["profiles"], ProfileView);
	        this.activeProfileId = source["activeProfileId"];
	        this.activeProvider = source["activeProvider"];
	        this.activeModel = source["activeModel"];
	        this.configPath = source["configPath"];
	        this.chatGptRunning = source["chatGptRunning"];
	        this.canRestore = source["canRestore"];
	        this.chatGptTarget = source["chatGptTarget"];
	        this.chatGptResolvedTarget = source["chatGptResolvedTarget"];
	        this.chatGptTargetError = source["chatGptTargetError"];
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
	export class CLIInfo {
	    id: string;
	    name: string;
	    command: string;
	    installed: boolean;
	    path: string;
	    error: string;

	    static createFrom(source: any = {}) {
	        return new CLIInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.command = source["command"];
	        this.installed = source["installed"];
	        this.path = source["path"];
	        this.error = source["error"];
	    }
	}
	export class CLIState {
	    tools: CLIInfo[];
	    directory: string;

	    static createFrom(source: any = {}) {
	        return new CLIState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tools = this.convertValues(source["tools"], CLIInfo);
	        this.directory = source["directory"];
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



	export class ProfileInput {
	    id: string;
	    name: string;
	    baseUrl: string;
	    apiKey: string;
	    clearApiKey: boolean;
	    selectedModel: string;
	    models: Model[];

	    static createFrom(source: any = {}) {
	        return new ProfileInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.clearApiKey = source["clearApiKey"];
	        this.selectedModel = source["selectedModel"];
	        this.models = this.convertValues(source["models"], Model);
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


	export class SkillCatalogItem {
	    id: string;
	    name: string;
	    directory: string;
	    description: string;
	    category: string;
	    source: string;
	    market: string;
	    repository: string;
	    url: string;
	    version: string;
	    author: string;
	    license: string;
	    hasScripts: boolean;
	    installed: boolean;
	    conflict: boolean;
	    skillPath: string;

	    static createFrom(source: any = {}) {
	        return new SkillCatalogItem(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.directory = source["directory"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.source = source["source"];
	        this.market = source["market"];
	        this.repository = source["repository"];
	        this.url = source["url"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.license = source["license"];
	        this.hasScripts = source["hasScripts"];
	        this.installed = source["installed"];
	        this.conflict = source["conflict"];
	        this.skillPath = source["skillPath"];
	    }
	}

	export class SkillMarket {
	    id: string;
	    name: string;
	    repository: string;

	    static createFrom(source: any = {}) {
	        return new SkillMarket(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.repository = source["repository"];
	    }
	}
	export class SkillPopularity {
	    name: string;
	    installs: number;
	    url: string;

	    static createFrom(source: any = {}) {
	        return new SkillPopularity(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.installs = source["installs"];
	        this.url = source["url"];
	    }
	}
	export class SkillMarketMetrics {
	    market: string;
	    repository: string;
	    repositoryStars?: number;
	    skills: SkillPopularity[];
	    updatedAt: string;
	    errors: string[];

	    static createFrom(source: any = {}) {
	        return new SkillMarketMetrics(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.market = source["market"];
	        this.repository = source["repository"];
	        this.repositoryStars = source["repositoryStars"];
	        this.skills = this.convertValues(source["skills"], SkillPopularity);
	        this.updatedAt = source["updatedAt"];
	        this.errors = source["errors"];
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

	export class SkillsState {
	    root: string;
	    target: string;
	    skills: SkillInfo[];
	    trash: SkillInfo[];
	    catalog: SkillCatalogItem[];
	    markets: SkillMarket[];
	    errors: string[];

	    static createFrom(source: any = {}) {
	        return new SkillsState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.target = source["target"];
	        this.skills = this.convertValues(source["skills"], SkillInfo);
	        this.trash = this.convertValues(source["trash"], SkillInfo);
	        this.catalog = this.convertValues(source["catalog"], SkillCatalogItem);
	        this.markets = this.convertValues(source["markets"], SkillMarket);
	        this.errors = source["errors"];
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
	export class UpdateInfo {
	    status: string;
	    message: string;
	    currentVersion: string;
	    latestVersion?: string;
	    notes?: string;
	    published?: string;
	    downloadUrl?: string;
	    sha256?: string;
	    signature?: string;
	    updateAvailable: boolean;

	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.notes = source["notes"];
	        this.published = source["published"];
	        this.downloadUrl = source["downloadUrl"];
	        this.sha256 = source["sha256"];
	        this.signature = source["signature"];
	        this.updateAvailable = source["updateAvailable"];
	    }
	}

	export class WorkspaceState {
	    projects: Workspace[];
	    activeId: string;

	    static createFrom(source: any = {}) {
	        return new WorkspaceState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projects = this.convertValues(source["projects"], Workspace);
	        this.activeId = source["activeId"];
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

