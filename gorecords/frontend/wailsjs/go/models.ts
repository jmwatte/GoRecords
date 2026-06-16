export namespace models {
	
	export class Track {
	    ID: number;
	    Path: string;
	    Title: string;
	    Artist: string;
	    AlbumArtist: string;
	    Album: string;
	    Genre: string;
	    Year: number;
	    TrackNumber: number;
	    DiscNumber: number;
	    Duration: number;
	    CoverPath: string;
	    AlbumFolder: string;
	
	    static createFrom(source: any = {}) {
	        return new Track(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Path = source["Path"];
	        this.Title = source["Title"];
	        this.Artist = source["Artist"];
	        this.AlbumArtist = source["AlbumArtist"];
	        this.Album = source["Album"];
	        this.Genre = source["Genre"];
	        this.Year = source["Year"];
	        this.TrackNumber = source["TrackNumber"];
	        this.DiscNumber = source["DiscNumber"];
	        this.Duration = source["Duration"];
	        this.CoverPath = source["CoverPath"];
	        this.AlbumFolder = source["AlbumFolder"];
	    }
	}

}

export namespace query {
	
	export class AlbumResult {
	    albumFolder: string;
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

