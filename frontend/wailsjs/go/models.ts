export namespace archive {
	
	export class Result {
	    zipPath: string;
	    pwdPath: string;
	    password: string;
	    encrypted: boolean;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.zipPath = source["zipPath"];
	        this.pwdPath = source["pwdPath"];
	        this.password = source["password"];
	        this.encrypted = source["encrypted"];
	        this.mode = source["mode"];
	    }
	}

}

export namespace config {
	
	export class Connection {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    dbName: string;
	    user: string;
	    sslMode: string;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.dbName = source["dbName"];
	        this.user = source["user"];
	        this.sslMode = source["sslMode"];
	    }
	}
	export class Config {
	    connections: Connection[];
	    selectedConnectionId: string;
	    dwellSeconds: number;
	    outputDir: string;
	    previewRows: number;
	    enforceReadOnly: boolean;
	    screenshots: boolean;
	    video: boolean;
	    monitorIndex: number;
	    stopOnError: boolean;
	    zip: boolean;
	    zipPasswordMode: string;
	    zipPassword: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connections = this.convertValues(source["connections"], Connection);
	        this.selectedConnectionId = source["selectedConnectionId"];
	        this.dwellSeconds = source["dwellSeconds"];
	        this.outputDir = source["outputDir"];
	        this.previewRows = source["previewRows"];
	        this.enforceReadOnly = source["enforceReadOnly"];
	        this.screenshots = source["screenshots"];
	        this.video = source["video"];
	        this.monitorIndex = source["monitorIndex"];
	        this.stopOnError = source["stopOnError"];
	        this.zip = source["zip"];
	        this.zipPasswordMode = source["zipPasswordMode"];
	        this.zipPassword = source["zipPassword"];
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

export namespace main {
	
	export class EnvInfo {
	    psqlFound: boolean;
	    psqlPath: string;
	    psqlVersion: string;
	    ffmpegFound: boolean;
	    numDisplays: number;
	    configDir: string;
	    screenAccess: boolean;
	    appVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.psqlFound = source["psqlFound"];
	        this.psqlPath = source["psqlPath"];
	        this.psqlVersion = source["psqlVersion"];
	        this.ffmpegFound = source["ffmpegFound"];
	        this.numDisplays = source["numDisplays"];
	        this.configDir = source["configDir"];
	        this.screenAccess = source["screenAccess"];
	        this.appVersion = source["appVersion"];
	    }
	}

}

export namespace store {
	
	export class Query {
	    id: string;
	    name: string;
	    sql: string;
	    enabled: boolean;
	    order: number;
	
	    static createFrom(source: any = {}) {
	        return new Query(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.sql = source["sql"];
	        this.enabled = source["enabled"];
	        this.order = source["order"];
	    }
	}

}

