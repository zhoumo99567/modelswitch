export namespace main {
	
	export class Model {
	    id: string;
	    owned_by?: string;
	
	    static createFrom(source: any = {}) {
	        return new Model(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.owned_by = source["owned_by"];
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
	    description: string;
	    category: string;
	    source: string;
	    url: string;
	    installed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SkillCatalogItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.source = source["source"];
	        this.url = source["url"];
	        this.installed = source["installed"];
	    }
	}
	export class SkillInfo {
	    name: string;
	    description: string;
	    path: string;
	    system: boolean;
	    hasScripts: boolean;
	    fileCount: number;
	    sizeBytes: number;
	    modifiedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.path = source["path"];
	        this.system = source["system"];
	        this.hasScripts = source["hasScripts"];
	        this.fileCount = source["fileCount"];
	        this.sizeBytes = source["sizeBytes"];
	        this.modifiedAt = source["modifiedAt"];
	    }
	}
	export class SkillsState {
	    root: string;
	    skills: SkillInfo[];
	    catalog: SkillCatalogItem[];
	
	    static createFrom(source: any = {}) {
	        return new SkillsState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.skills = this.convertValues(source["skills"], SkillInfo);
	        this.catalog = this.convertValues(source["catalog"], SkillCatalogItem);
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

}

