export namespace main {

	export class ArsipinConfigDTO {
	    uploadUrl: string;
	    passwordConfigured: boolean;

	    static createFrom(source: any = {}) {
	        return new ArsipinConfigDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uploadUrl = source["uploadUrl"];
	        this.passwordConfigured = source["passwordConfigured"];
	    }
	}
	export class ArsipinUploadResultDTO {
	    success: boolean;
	    message: string;
	    statusCode: number;
	    errorCode: string;
	    archiveId: string;
	    jobId: string;

	    static createFrom(source: any = {}) {
	        return new ArsipinUploadResultDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.statusCode = source["statusCode"];
	        this.errorCode = source["errorCode"];
	        this.archiveId = source["archiveId"];
	        this.jobId = source["jobId"];
	    }
	}
	export class ExportResultDTO {
	    cancelled: boolean;
	    format: string;
	    paths: string[];

	    static createFrom(source: any = {}) {
	        return new ExportResultDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cancelled = source["cancelled"];
	        this.format = source["format"];
	        this.paths = source["paths"];
	    }
	}
	export class PageDTO {
	    id: string;
	    thumbnailDataURL: string;
	    selected: boolean;
	    selectionOrder: number;
	    width: number;
	    height: number;

	    static createFrom(source: any = {}) {
	        return new PageDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.thumbnailDataURL = source["thumbnailDataURL"];
	        this.selected = source["selected"];
	        this.selectionOrder = source["selectionOrder"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	export class ScanRequest {
	    scannerId: string;
	    mode: string;
	    dpi: number;

	    static createFrom(source: any = {}) {
	        return new ScanRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scannerId = source["scannerId"];
	        this.mode = source["mode"];
	        this.dpi = source["dpi"];
	    }
	}
	export class SessionDTO {
	    pages: PageDTO[];
	    selectedCount: number;
	    status: string;

	    static createFrom(source: any = {}) {
	        return new SessionDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pages = this.convertValues(source["pages"], PageDTO);
	        this.selectedCount = source["selectedCount"];
	        this.status = source["status"];
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
	export class ScanResultDTO {
	    session: SessionDTO;
	    warnings: string[];

	    static createFrom(source: any = {}) {
	        return new ScanResultDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session = this.convertValues(source["session"], SessionDTO);
	        this.warnings = source["warnings"];
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
	export class ScannerDTO {
	    id: string;
	    name: string;

	    static createFrom(source: any = {}) {
	        return new ScannerDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}

	export class UpdateDownloadDTO {
	    version: string;
	    path: string;

	    static createFrom(source: any = {}) {
	        return new UpdateDownloadDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.path = source["path"];
	    }
	}
	export class UpdateInfoDTO {
	    available: boolean;
	    currentVersion: string;
	    latestVersion: string;
	    releaseName: string;
	    releaseNotes: string;

	    static createFrom(source: any = {}) {
	        return new UpdateInfoDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.releaseName = source["releaseName"];
	        this.releaseNotes = source["releaseNotes"];
	    }
	}

}
