export namespace player {
	
	export class Binaries {
	    mpv: string;
	    ytdlp: string;
	
	    static createFrom(source: any = {}) {
	        return new Binaries(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mpv = source["mpv"];
	        this.ytdlp = source["ytdlp"];
	    }
	}
	export class Hotkey {
	    keys: string;
	    action: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Hotkey(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keys = source["keys"];
	        this.action = source["action"];
	        this.active = source["active"];
	    }
	}
	export class RecentItem {
	    url: string;
	    kind: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new RecentItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	    }
	}
	export class Source {
	    url: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new Source(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.kind = source["kind"];
	    }
	}
	export class SubTrack {
	    id: number;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new SubTrack(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	    }
	}
	export class State {
	    status: string;
	    url: string;
	    kind: string;
	    title: string;
	    paused: boolean;
	    buffering: boolean;
	    volume: number;
	    muted: boolean;
	    onTop: boolean;
	    size: string;
	    error: string;
	    position: number;
	    duration: number;
	    seekable: boolean;
	    live: boolean;
	    speed: number;
	    subTracks: SubTrack[];
	    sub: number;
	    quality: string;
	    opacity: number;
	    clickThrough: boolean;
	    hidden: boolean;
	    recent: RecentItem[];
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.url = source["url"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.paused = source["paused"];
	        this.buffering = source["buffering"];
	        this.volume = source["volume"];
	        this.muted = source["muted"];
	        this.onTop = source["onTop"];
	        this.size = source["size"];
	        this.error = source["error"];
	        this.position = source["position"];
	        this.duration = source["duration"];
	        this.seekable = source["seekable"];
	        this.live = source["live"];
	        this.speed = source["speed"];
	        this.subTracks = this.convertValues(source["subTracks"], SubTrack);
	        this.sub = source["sub"];
	        this.quality = source["quality"];
	        this.opacity = source["opacity"];
	        this.clickThrough = source["clickThrough"];
	        this.hidden = source["hidden"];
	        this.recent = this.convertValues(source["recent"], RecentItem);
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

