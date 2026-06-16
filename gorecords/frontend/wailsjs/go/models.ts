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

