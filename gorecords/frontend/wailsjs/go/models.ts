export namespace models {
	
	export class Track {
	    id: number;
	    // Go type: time
	    dateAdded: any;
	    path: string;
	    title: string;
	    artist: string;
	    albumArtist: string;
	    album: string;
	    genre: string;
	    year: number;
	    trackNumber: number;
	    discNumber: number;
	    duration: number;
	    coverPath: string;
	    albumFolder: string;
	
	    static createFrom(source: any = {}) {
	        return new Track(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.dateAdded = this.convertValues(source["dateAdded"], null);
	        this.path = source["path"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.albumArtist = source["albumArtist"];
	        this.album = source["album"];
	        this.genre = source["genre"];
	        this.year = source["year"];
	        this.trackNumber = source["trackNumber"];
	        this.discNumber = source["discNumber"];
	        this.duration = source["duration"];
	        this.coverPath = source["coverPath"];
	        this.albumFolder = source["albumFolder"];
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

export namespace query {
	
	export class AlbumResult {
	    albumFolder: string;
	    dateAdded: string;
	    album: string;
	    albumArtist: string;
	    coverPath: string;
	    year: number;
	    genre: string;
	    trackCount: number;
	    totalDuration: number;
	
	    static createFrom(source: any = {}) {
	        return new AlbumResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.albumFolder = source["albumFolder"];
	        this.dateAdded = source["dateAdded"];
	        this.album = source["album"];
	        this.albumArtist = source["albumArtist"];
	        this.coverPath = source["coverPath"];
	        this.year = source["year"];
	        this.genre = source["genre"];
	        this.trackCount = source["trackCount"];
	        this.totalDuration = source["totalDuration"];
	    }
	}
	export class PaginatedAlbums {
	    albums: AlbumResult[];
	    total: number;
	    offset: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new PaginatedAlbums(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.albums = this.convertValues(source["albums"], AlbumResult);
	        this.total = source["total"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
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

