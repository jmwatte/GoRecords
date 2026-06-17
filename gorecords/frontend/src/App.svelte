<script>
    import { onMount } from "svelte";
    import { initKeyboard, lastAction } from "./lib/keyboard.js";
    import {
        viewMode,
        currentIndex,
        currentView,
        isVisualMode,
        isAlbumTracksView,
        toggleViewMode,
        openAlbum,
        closeAlbum,
        activeAlbumFolder,
        scanProgress,
        filterStack,
        breadcrumbIndex,
        activePicker,
        pushFilter,
        popFilter,
        clearFilters,
        filtersToPayload,
        facetData,
    } from "./lib/stores.js";
    import {
        GetAlbumTracks,
        GetRandomAlbum,
        GetAlbums,
        GetFilteredAlbums,
        GetFacets,
    } from "../wailsjs/go/main/App.js";
    import Settings from "./lib/Settings.svelte";
    import FilterPicker from "./lib/FilterPicker.svelte";
    import Toasts from "./lib/Toasts.svelte";
    import { showToast } from "./lib/toastStore.js";

    let cleanup;

    // Album list — fetched from backend on mount
    let albums = [];
    let totalAlbums = 0;
    let loadingAlbums = true;

    // Track list — populated via Wails binding when album is opened
    let tracks = [];
    let loadingTracks = false;

    // Cached index to restore on Escape
    let crateIndexSnapshot = 0;

    // Override openAlbum to fetch tracks from backend
    async function handleOpenAlbum(albumFolder) {
        crateIndexSnapshot = $currentIndex;
        openAlbum(albumFolder);
        loadingTracks = true;
        try {
            tracks = (await GetAlbumTracks(albumFolder)) || [];
        } catch (err) {
            console.error("Failed to fetch tracks:", err);
            showToast("Failed to load album tracks", "error");
            tracks = [];
        } finally {
            loadingTracks = false;
        }
    }

    // Override closeAlbum to restore crate index
    function handleCloseAlbum() {
        closeAlbum();
        $currentIndex = crateIndexSnapshot;
    }

    onMount(() => {
        cleanup = initKeyboard();
        refreshData();
        return () => {
            if (cleanup) cleanup();
        };
    });

    // Fetch albums and facets from the backend
    async function refreshData() {
        loadingAlbums = true;
        const payload = filtersToPayload($filterStack);
        try {
            const [albumResult, facetResult] = await Promise.all([
                GetFilteredAlbums(payload, 0, 1000),
                GetFacets(payload),
            ]);
            albums = albumResult.albums || [];
            totalAlbums = albumResult.total || albums.length;
            facetData.set(facetResult || {});
        } catch (err) {
            console.error("Failed to load data:", err);
            showToast("Failed to load data", "error");
            albums = [];
            totalAlbums = 0;
        } finally {
            loadingAlbums = false;
        }
    }

    // Reload albums and facets when scan completes
    $: if (cleanup && $scanProgress === -1) {
        refreshData();
    }

    // Reload albums and facets when filters change
    let prevFilterLen = 0;
    $: if (cleanup && $filterStack) {
        if ($filterStack.length !== prevFilterLen) {
            prevFilterLen = $filterStack.length;
            refreshData();
        }
    }

    // Subscribe to matched keyboard actions for navigation.
    // $lastAction.seq changes on every keydown so this always fires.
    $: if ($lastAction.action) {
        handleAction($lastAction.action);
    }

    // Auto-scroll to keep the selected track visible
    let tracksBodyEl;
    let prevIndex = -1;
    $: if (
        tracksBodyEl &&
        $isAlbumTracksView &&
        $currentIndex >= 0 &&
        $currentIndex !== prevIndex
    ) {
        prevIndex = $currentIndex;
        const sel = tracksBodyEl.querySelector(".track-row.selected");
        if (sel) {
            sel.scrollIntoView({ block: "center", behavior: "smooth" });
        }
    }

    // Also scroll to keep currentIndex visible when entering a new album
    let prevAlbumFolder = "";
    $: if ($activeAlbumFolder && $activeAlbumFolder !== prevAlbumFolder) {
        prevAlbumFolder = $activeAlbumFolder;
        // Reset scroll when entering a new album
        if (tracksBodyEl) {
            tracksBodyEl.scrollTop = 0;
        }
    }

    function handleAction(action) {
        // Global shortcuts (work in any view)
        switch (action) {
            case "rewind":
                handleRewind();
                return;
            case "play_pause":
                handlePlayPause();
                return;
            case "next_track":
                nextTrack();
                return;
            case "prev_track":
                prevTrack();
                return;
        }

        if ($isAlbumTracksView) {
            // Navigation within album track list
            switch (action) {
                case "nav_up":
                    $currentIndex = Math.max(0, $currentIndex - 1);
                    break;
                case "nav_down":
                    $currentIndex = Math.min(
                        tracks.length - 1,
                        $currentIndex + 1,
                    );
                    break;
                case "nav_left":
                case "close_view":
                case "go_back":
                    handleCloseAlbum();
                    break;
                case "open_album":
                    handlePlaySelected();
                    break;
            }
            return;
        }

        // Crate-level navigation
        switch (action) {
            case "nav_up":
                $currentIndex = Math.max(0, $currentIndex - 1);
                $breadcrumbIndex = -1;
                break;
            case "nav_down":
                $currentIndex = Math.min(totalAlbums - 1, $currentIndex + 1);
                $breadcrumbIndex = -1;
                break;
            case "breadcrumb_prev":
                if ($filterStack.length > 0) {
                    $breadcrumbIndex = Math.max(
                        0,
                        ($breadcrumbIndex >= 0
                            ? $breadcrumbIndex
                            : $filterStack.length) - 1,
                    );
                }
                break;
            case "breadcrumb_next":
                if ($filterStack.length > 0) {
                    $breadcrumbIndex = Math.min(
                        $filterStack.length - 1,
                        ($breadcrumbIndex >= 0 ? $breadcrumbIndex : -1) + 1,
                    );
                }
                break;
            case "open_album":
                // Enter: open album, or re-open picker when breadcrumb highlighted
                if (
                    $breadcrumbIndex >= 0 &&
                    $breadcrumbIndex < $filterStack.length
                ) {
                    const cat = $filterStack[$breadcrumbIndex].category;
                    popFilter($breadcrumbIndex);
                    $breadcrumbIndex = -1;
                    $activePicker = cat;
                } else if (albums[$currentIndex]) {
                    handleOpenAlbum(albums[$currentIndex].albumFolder);
                }
                break;
            case "enter_album":
                // ArrowRight: just open album, no breadcrumb interference
                if (albums[$currentIndex]) {
                    handleOpenAlbum(albums[$currentIndex].albumFolder);
                }
                break;
            case "visual_mode":
                if (!$isVisualMode) toggleViewMode();
                break;
            case "list_mode":
                if ($isVisualMode) toggleViewMode();
                break;
            case "settings":
                $currentView =
                    $currentView === "settings" ? "crate" : "settings";
                break;
            case "random_album":
                handleRandomAlbum();
                break;
            case "toggle_picker":
                // b: toggle the add filter picker
                if ($activePicker) {
                    activePicker.set(null);
                } else {
                    $breadcrumbIndex = -1;
                    $activePicker = "add";
                }
                break;
        }
    }

    // Helper: format seconds to mm:ss
    function fmtDuration(sec) {
        if (!sec || sec <= 0) return "--:--";
        const m = Math.floor(sec / 60);
        const s = Math.floor(sec % 60);
        return `${m}:${s.toString().padStart(2, "0")}`;
    }

    // Group tracks by disc for rendering with separators
    $: discGroups = (() => {
        const groups = [];
        let currentDisc = null;
        let currentGroup = [];
        for (const t of tracks) {
            if (t.discNumber !== currentDisc) {
                if (currentGroup.length) {
                    groups.push({ disc: currentDisc, tracks: currentGroup });
                }
                currentDisc = t.discNumber;
                currentGroup = [t];
            } else {
                currentGroup.push(t);
            }
        }
        if (currentGroup.length) {
            groups.push({ disc: currentDisc, tracks: currentGroup });
        }
        return groups;
    })();

    // Audio playback state
    let audioEl;
    let currentTrackPath = "";
    let isPlaying = false;
    let nowPlaying = null; // { title, artist, albumArtist, coverPath }
    let currentTime = 0;
    let duration = 0;
    let volume = 1;

    // Sync with audio element events
    function onTimeUpdate() {
        if (audioEl) {
            currentTime = audioEl.currentTime;
            duration = audioEl.duration || 0;
        }
    }

    function onLoadedMetadata() {
        if (audioEl) {
            duration = audioEl.duration || 0;
        }
    }

    function onSeek(e) {
        if (audioEl) {
            audioEl.currentTime = parseFloat(e.target.value);
        }
    }

    function onVolumeChange(e) {
        if (audioEl) {
            audioEl.volume = parseFloat(e.target.value);
            volume = audioEl.volume;
        }
    }

    // Compute the /media/ URL for a track path, URL-encoded for safety
    function audioSrc(trackPath) {
        if (!trackPath) return "";
        return "/media/" + encodeURIComponent(trackPath);
    }

    // Play the track at the given index in the current tracks list
    function playTrack(idx) {
        if (!tracks[idx]) return;
        $currentIndex = idx;
        const track = tracks[idx];
        if (track.path && audioEl) {
            currentTrackPath = track.path;
            nowPlaying = {
                title: track.title,
                artist: track.artist,
                albumArtist: track.albumArtist,
                coverPath: track.coverPath,
            };
            audioEl.src = audioSrc(track.path);
            audioEl.play().catch((err) => {
                console.error("Playback failed:", err);
                showToast("Playback failed for this track", "warn");
            });
            isPlaying = true;
            updateMediaSession(track);
        }
    }

    // Play selected track on Enter in album_tracks view
    function handlePlaySelected() {
        if (
            $isAlbumTracksView &&
            tracks.length > 0 &&
            $currentIndex >= 0 &&
            $currentIndex < tracks.length
        ) {
            playTrack($currentIndex);
        }
    }

    // Auto-advance to next track when current one ends
    function onTrackEnded() {
        const next = $currentIndex + 1;
        if (tracks.length > 0 && next < tracks.length) {
            playTrack(next);
        } else {
            isPlaying = false;
        }
    }

    // Toggle play/pause for the currently loaded track
    function handlePlayPause() {
        if (!audioEl || !currentTrackPath) return;
        if (audioEl.paused) {
            audioEl
                .play()
                .catch((err) => console.error("Playback failed:", err));
        } else {
            audioEl.pause();
        }
    }

    function prevTrack() {
        const prev = $currentIndex - 1;
        if (prev >= 0) playTrack(prev);
    }

    function nextTrack() {
        const next = $currentIndex + 1;
        if (next < tracks.length) playTrack(next);
    }

    function toggleMute() {
        if (audioEl) {
            if (audioEl.volume > 0) {
                audioEl.volume = 0;
                volume = 0;
            } else {
                audioEl.volume = 1;
                volume = 1;
            }
        }
    }

    function handleRewind() {
        if (audioEl) {
            audioEl.currentTime = Math.max(0, audioEl.currentTime - 2);
        }
    }

    // Pick a random album and select it in the crate view (no navigation)
    async function handleRandomAlbum() {
        if ($currentView !== "crate") return;
        try {
            const payload = filtersToPayload($filterStack);
            const albumFolder = await GetRandomAlbum(payload);
            if (albumFolder) {
                const idx = albums.findIndex(
                    (a) => a.albumFolder === albumFolder,
                );
                if (idx >= 0) {
                    $currentIndex = idx;
                    showToast("Random album selected", "info", 2000);
                } else {
                    showToast("Random album not in current view", "warn", 2000);
                }
            } else {
                showToast("No albums found", "warn", 2000);
            }
        } catch (err) {
            console.error("Random album failed:", err);
            showToast("Failed to pick random album", "error");
        }
    }

    // Media Session API: push metadata and action handlers to the OS media bar
    function updateMediaSession(track) {
        if (!("mediaSession" in navigator)) return;

        const albumInfo = albums.find(
            (a) => a.albumFolder === $activeAlbumFolder,
        );

        navigator.mediaSession.metadata = new MediaMetadata({
            title: track.title || "Unknown Track",
            artist: track.artist || track.albumArtist || "Unknown Artist",
            album: track.album || albumInfo?.album || "Unknown Album",
            artwork: track.coverPath
                ? [
                      {
                          src: audioSrc(track.coverPath),
                          sizes: "512x512",
                          type: "image/jpeg",
                      },
                  ]
                : [],
        });

        navigator.mediaSession.setActionHandler("play", () => {
            audioEl?.play().catch(() => {});
        });
        navigator.mediaSession.setActionHandler("pause", () => {
            audioEl?.pause();
        });
        navigator.mediaSession.setActionHandler("previoustrack", () => {
            const prev = $currentIndex - 1;
            if (prev >= 0) playTrack(prev);
        });
        navigator.mediaSession.setActionHandler("nexttrack", () => {
            const next = $currentIndex + 1;
            if (next < tracks.length) playTrack(next);
        });
    }
</script>

<div class="app-shell">
    {#if $currentView === "crate"}
        <div class="crate-layout">
            <div class="crate-content">
                <!-- Breadcrumb bar: filter stack chips + add button -->
                <div class="breadcrumb-bar">
                    {#each $filterStack as filter, i}
                        <button
                            class="breadcrumb-chip"
                            class:breadcrumb-active={i === $breadcrumbIndex}
                            on:click={() => {
                                $breadcrumbIndex = i;
                            }}
                            on:dblclick={() => {
                                const cat = filter.category;
                                popFilter(i);
                                $breadcrumbIndex = -1;
                                $activePicker = cat;
                            }}
                        >
                            <span class="breadcrumb-label"
                                >{filter.category}:</span
                            >
                            <span class="breadcrumb-value">{filter.value}</span>
                            <span
                                class="breadcrumb-remove"
                                on:click|stopPropagation={() => popFilter(i)}
                                >✕</span
                            >
                        </button>
                    {/each}
                    <button
                        class="breadcrumb-add"
                        on:click={() => {
                            $activePicker = "add";
                            $breadcrumbIndex = -1;
                        }}
                        title="Add filter">+</button
                    >
                    {#if $filterStack.length > 0}
                        <button
                            class="breadcrumb-clear"
                            on:click={() => {
                                clearFilters();
                                $breadcrumbIndex = -1;
                            }}>Clear all</button
                        >
                    {/if}
                </div>

                {#if $isVisualMode}
                    <div class="visual-crate">
                        <div class="album-art-frame">
                            {#if albums[$currentIndex]?.coverPath}
                                <img
                                    class="album-art"
                                    src={audioSrc(
                                        albums[$currentIndex].coverPath,
                                    )}
                                    alt={albums[$currentIndex].album}
                                />
                            {:else}
                                <img
                                    class="album-art"
                                    src="/no_results.png"
                                    alt={albums[$currentIndex]?.album ??
                                        "Album"}
                                />
                            {/if}
                        </div>
                        <div class="album-meta">
                            <h1 class="album-title">
                                {albums[$currentIndex]?.album ?? "—"}
                            </h1>
                            <h2 class="album-artist">
                                {albums[$currentIndex]?.albumArtist ?? "—"}
                            </h2>
                            <span class="album-year"
                                >{albums[$currentIndex]?.year ?? ""}</span
                            >
                            <span class="sep">·</span>
                            <span class="album-genre"
                                >{albums[$currentIndex]?.genre ?? ""}</span
                            >
                        </div>
                        <div class="nav-hint">
                            ← {$currentIndex + 1} / {totalAlbums} →
                        </div>
                    </div>
                {:else}
                    <div class="text-crate">
                        <div class="ledger-header">
                            <span class="col-artist">Artist</span>
                            <span class="col-album">Album</span>
                            <span class="col-year">Year</span>
                            <span class="col-genre">Genre</span>
                            <span class="col-tracks">Tracks</span>
                        </div>
                        <div class="ledger-body">
                            {#each albums as album, i}
                                <div
                                    class="ledger-row"
                                    class:selected={i === $currentIndex}
                                    on:click={() =>
                                        handleOpenAlbum(album.albumFolder)}
                                    on:dblclick={() =>
                                        handleOpenAlbum(album.albumFolder)}
                                >
                                    <span class="col-artist"
                                        >{album.albumArtist}</span
                                    >
                                    <span class="col-album">{album.album}</span>
                                    <span class="col-year">{album.year}</span>
                                    <span class="col-genre">{album.genre}</span>
                                    <span class="col-tracks"
                                        >{album.trackCount}</span
                                    >
                                </div>
                            {/each}
                        </div>
                    </div>
                {/if}
            </div>
        </div>
    {:else if $currentView === "album_tracks"}
        <div class="album-tracks-view">
            <button class="back-btn" on:click={handleCloseAlbum}
                >← Back to Crate</button
            >
            <h2 class="tracks-album-title">
                {albums.find((a) => a.albumFolder === $activeAlbumFolder)
                    ?.album ?? "Album"}
                <span class="tracks-count">· {tracks.length} tracks</span>
            </h2>

            {#if loadingTracks}
                <div class="loading">Loading tracks…</div>
            {:else if tracks.length === 0}
                <div class="loading">No tracks found.</div>
            {:else}
                <div class="tracks-header">
                    <span class="col-track">#</span>
                    <span class="col-disc">Disc</span>
                    <span class="col-title">Title</span>
                    <span class="col-artist">Artist</span>
                    <span class="col-duration">Time</span>
                </div>
                <div class="tracks-body" bind:this={tracksBodyEl}>
                    {#each discGroups as group}
                        <div class="disc-separator">Disc {group.disc}</div>
                        {#each group.tracks as track}
                            {@const idx = tracks.indexOf(track)}
                            <div
                                class="track-row"
                                class:selected={idx === $currentIndex}
                                on:click={() => playTrack(idx)}
                                on:dblclick={() => playTrack(idx)}
                            >
                                <span class="col-track"
                                    >{track.trackNumber}</span
                                >
                                <span class="col-disc"
                                    >{track.discNumber > 0
                                        ? track.discNumber
                                        : ""}</span
                                >
                                <span class="col-title"
                                    >{track.title ||
                                        track.path?.split(/[\\/]/).pop()}</span
                                >
                                <span class="col-artist">{track.artist}</span>
                                <span class="col-duration"
                                    >{fmtDuration(track.duration)}</span
                                >
                            </div>
                        {/each}
                    {/each}
                </div>
            {/if}
        </div>
    {:else if $currentView === "settings"}
        <Settings />
    {/if}

    <!-- Filter Picker overlays -->
    {#if $activePicker === "add"}
        <!-- Category chooser when user clicks + -->
        <div class="picker-overlay">
            <div class="picker-panel">
                <div class="picker-header">
                    <h3 class="picker-title">Pick a category</h3>
                    <button
                        class="picker-close"
                        on:click={() => activePicker.set(null)}>✕</button
                    >
                </div>
                <div class="picker-body">
                    {#each ["genre", "year", "artist"] as cat}
                        <button
                            class="picker-item"
                            on:click={() => ($activePicker = cat)}
                        >
                            <span
                                class="picker-item-value"
                                style="text-transform:capitalize">{cat}</span
                            >
                        </button>
                    {/each}
                </div>
            </div>
        </div>
    {:else if $activePicker}
        <FilterPicker category={$activePicker} />
    {/if}

    <!-- Fixed Now Playing bar -->
    <div class="now-playing-bar">
        <div class="np-track-info">
            {#if nowPlaying}
                <span class="np-title"
                    >{nowPlaying.title || "Unknown Track"}</span
                >
                <span class="np-artist"
                    >{nowPlaying.artist ||
                        nowPlaying.albumArtist ||
                        "Unknown Artist"}</span
                >
            {:else}
                <span class="np-title np-muted">No track selected</span>
                <span class="np-artist np-muted">Select a track to play</span>
            {/if}
        </div>

        <div class="np-controls">
            <button class="np-btn" on:click={prevTrack} title="Previous (p)"
                >⏮</button
            >
            <button
                class="np-btn np-play-btn"
                on:click={handlePlayPause}
                title="Play/Pause (Space)"
            >
                {isPlaying ? "⏸" : "▶"}
            </button>
            <button class="np-btn" on:click={nextTrack} title="Next (n)"
                >⏭</button
            >
        </div>

        <div class="np-seek">
            <span class="np-time">{fmtDuration(currentTime)}</span>
            <input
                type="range"
                class="np-slider"
                min="0"
                max={duration || 0}
                step="0.1"
                value={currentTime}
                on:input={onSeek}
            />
            <span class="np-time">{fmtDuration(duration)}</span>
        </div>

        <div class="np-volume">
            <button class="np-btn" on:click={toggleMute} title="Mute (m)">
                {volume === 0 ? "🔇" : volume < 0.5 ? "🔉" : "🔊"}
            </button>
            <input
                type="range"
                class="np-slider np-volume-slider"
                min="0"
                max="1"
                step="0.01"
                bind:value={volume}
                on:input={onVolumeChange}
            />
        </div>
    </div>

    <!-- Toast notifications -->
    <Toasts />

    <!-- Hidden audio element for playback -->
    <audio
        bind:this={audioEl}
        preload="auto"
        on:ended={onTrackEnded}
        on:play={() => (isPlaying = true)}
        on:pause={() => (isPlaying = false)}
        on:timeupdate={onTimeUpdate}
        on:loadedmetadata={onLoadedMetadata}
    ></audio>
</div>

<style>
    .app-shell {
        width: 100%;
        height: 100vh;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    /* ===== Crate Layout with Sidebar ===== */
    .crate-layout {
        flex: 1;
        display: flex;
        overflow: hidden;
    }

    .crate-content {
        flex: 1;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    /* ===== Visual Mode ===== */
    .visual-crate {
        flex: 1;
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 1rem;
        padding: 1rem 1rem 0;
        user-select: none;
        min-height: 0;
        overflow: hidden;
    }

    .album-art-frame {
        width: 100%;
        max-width: min(80vh, 800px);
        aspect-ratio: 1 / 1;
        border-radius: 8px;
        overflow: hidden;
        box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
        flex-shrink: 0;
    }

    .album-art {
        width: 100%;
        height: 100%;
        object-fit: cover;
        display: block;
    }

    .album-art-placeholder {
        width: 100%;
        height: 100%;
        background: linear-gradient(135deg, #2a3a5c, #1a2a4a);
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .placeholder-text {
        font-size: 6rem;
        font-weight: 700;
        color: rgba(255, 255, 255, 0.15);
    }

    .album-meta {
        text-align: center;
    }

    .album-title {
        margin: 0;
        font-size: 1.6rem;
        font-weight: 600;
    }

    .album-artist {
        margin: 0.25rem 0 0.5rem;
        font-size: 1.1rem;
        font-weight: 400;
        color: rgba(255, 255, 255, 0.6);
    }

    .sep {
        margin: 0 0.5rem;
        color: rgba(255, 255, 255, 0.3);
    }

    .album-year,
    .album-genre {
        font-size: 0.9rem;
        color: rgba(255, 255, 255, 0.5);
    }

    .nav-hint {
        font-size: 0.8rem;
        color: rgba(255, 255, 255, 0.3);
        letter-spacing: 0.1em;
    }

    /* ===== Text Mode (Ledger) ===== */
    .text-crate {
        flex: 1;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        user-select: none;
    }

    .ledger-header {
        display: flex;
        padding: 0.5rem 1rem;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 0.1em;
        color: rgba(255, 255, 255, 0.4);
    }

    .ledger-body {
        flex: 1;
        overflow-y: auto;
    }

    .ledger-row {
        display: flex;
        padding: 0.4rem 1rem;
        border-bottom: 1px solid rgba(255, 255, 255, 0.04);
        cursor: pointer;
        transition: background 0.1s;
        font-size: 0.9rem;
    }

    .ledger-row:hover {
        background: rgba(255, 255, 255, 0.05);
    }

    .ledger-row.selected {
        background: rgba(255, 255, 255, 0.12);
        outline: 1px solid rgba(255, 255, 255, 0.2);
    }

    .col-artist {
        flex: 2;
    }
    .col-album {
        flex: 3;
    }
    .col-year {
        flex: 0 0 60px;
    }
    .col-genre {
        flex: 1;
    }
    .col-tracks {
        flex: 0 0 60px;
        text-align: right;
    }

    /* ===== Album Tracks View ===== */
    .album-tracks-view {
        flex: 1;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        padding: 1rem;
        user-select: none;
    }

    .back-btn {
        align-self: flex-start;
        background: none;
        border: 1px solid rgba(255, 255, 255, 0.2);
        color: white;
        padding: 0.3rem 0.8rem;
        border-radius: 4px;
        cursor: pointer;
        margin-bottom: 0.75rem;
        font-size: 0.85rem;
    }

    .back-btn:hover {
        background: rgba(255, 255, 255, 0.1);
    }

    .tracks-album-title {
        margin: 0 0 0.5rem;
        font-size: 1.3rem;
        font-weight: 600;
    }

    .tracks-count {
        font-size: 0.85rem;
        font-weight: 400;
        color: rgba(255, 255, 255, 0.4);
    }

    .loading {
        flex: 1;
        display: flex;
        align-items: center;
        justify-content: center;
        color: rgba(255, 255, 255, 0.4);
        font-size: 0.9rem;
    }

    .tracks-header {
        display: flex;
        padding: 0.4rem 0.5rem;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
        font-size: 0.7rem;
        text-transform: uppercase;
        letter-spacing: 0.1em;
        color: rgba(255, 255, 255, 0.4);
    }

    .tracks-body {
        flex: 1;
        overflow-y: auto;
    }

    .disc-separator {
        padding: 0.5rem 0.5rem 0.25rem;
        font-size: 0.75rem;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.08em;
        color: rgba(255, 255, 255, 0.5);
        border-bottom: 1px solid rgba(255, 255, 255, 0.06);
        margin-top: 0.5rem;
    }

    .disc-separator:first-child {
        margin-top: 0;
    }

    .track-row {
        display: flex;
        padding: 0.35rem 0.5rem;
        border-bottom: 1px solid rgba(255, 255, 255, 0.04);
        font-size: 0.85rem;
        transition: background 0.1s;
        cursor: pointer;
    }

    .track-row:hover {
        background: rgba(255, 255, 255, 0.05);
    }

    .track-row.selected {
        background: rgba(255, 255, 255, 0.12);
        outline: 1px solid rgba(255, 255, 255, 0.2);
    }

    .col-track {
        flex: 0 0 40px;
    }
    .col-disc {
        flex: 0 0 40px;
    }
    .col-title {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .col-artist {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        color: rgba(255, 255, 255, 0.5);
    }
    .col-duration {
        flex: 0 0 60px;
        text-align: right;
    }

    /* ===== Now Playing Bar ===== */
    .now-playing-bar {
        flex-shrink: 0;
        display: flex;
        align-items: center;
        gap: 1rem;
        padding: 0.5rem 1rem;
        background: rgba(0, 0, 0, 0.4);
        border-top: 1px solid rgba(255, 255, 255, 0.08);
        user-select: none;
        height: 56px;
        min-height: 56px;
    }

    .np-track-info {
        flex: 0 0 200px;
        display: flex;
        flex-direction: column;
        min-width: 0;
    }

    .np-title {
        font-size: 0.85rem;
        font-weight: 600;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .np-artist {
        font-size: 0.75rem;
        color: rgba(255, 255, 255, 0.5);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .np-muted {
        color: rgba(255, 255, 255, 0.25);
        font-style: italic;
    }

    .np-controls {
        display: flex;
        align-items: center;
        gap: 0.25rem;
    }

    .np-btn {
        background: none;
        border: none;
        color: rgba(255, 255, 255, 0.7);
        font-size: 1.1rem;
        padding: 0.25rem 0.4rem;
        cursor: pointer;
        border-radius: 4px;
        line-height: 1;
        transition:
            color 0.1s,
            background 0.1s;
    }

    .np-btn:hover {
        color: white;
        background: rgba(255, 255, 255, 0.1);
    }

    .np-play-btn {
        font-size: 1.3rem;
        padding: 0.25rem 0.6rem;
    }

    .np-seek {
        flex: 1;
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .np-time {
        font-size: 0.7rem;
        color: rgba(255, 255, 255, 0.5);
        font-variant-numeric: tabular-nums;
        flex-shrink: 0;
        min-width: 4ch;
        text-align: center;
    }

    .np-slider {
        flex: 1;
        -webkit-appearance: none;
        appearance: none;
        height: 4px;
        border-radius: 2px;
        background: rgba(255, 255, 255, 0.15);
        outline: none;
        cursor: pointer;
    }

    .np-slider::-webkit-slider-thumb {
        -webkit-appearance: none;
        appearance: none;
        width: 12px;
        height: 12px;
        border-radius: 50%;
        background: rgba(70, 130, 200, 0.8);
        cursor: pointer;
        transition: transform 0.1s;
    }

    .np-slider::-webkit-slider-thumb:hover {
        transform: scale(1.2);
    }

    .np-slider::-moz-range-thumb {
        width: 12px;
        height: 12px;
        border-radius: 50%;
        background: rgba(70, 130, 200, 0.8);
        cursor: pointer;
        border: none;
    }

    .np-volume {
        flex: 0 0 120px;
        display: flex;
        align-items: center;
        gap: 0.3rem;
    }

    .np-volume-slider {
        flex: 1;
    }

    /* Breadcrumb filter chips */
    .breadcrumb-bar {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: 0.4rem;
        padding: 0.5rem 0.75rem;
        border-bottom: 1px solid rgba(255, 255, 255, 0.06);
    }

    .breadcrumb-chip {
        display: inline-flex;
        align-items: center;
        gap: 0.3rem;
        background: rgba(255, 255, 255, 0.06);
        border: 1px solid rgba(255, 255, 255, 0.12);
        border-radius: 4px;
        padding: 0.25rem 0.5rem;
        font-size: 0.75rem;
        color: white;
        cursor: pointer;
        transition: all 0.1s;
    }

    .breadcrumb-chip:hover {
        background: rgba(70, 130, 200, 0.2);
        border-color: rgba(70, 130, 200, 0.4);
    }

    .breadcrumb-active {
        background: rgba(70, 130, 200, 0.3) !important;
        border-color: rgba(70, 130, 200, 0.7) !important;
    }

    .breadcrumb-label {
        color: rgba(255, 255, 255, 0.5);
        text-transform: uppercase;
        font-size: 0.65rem;
    }

    .breadcrumb-value {
        font-weight: 500;
    }

    .breadcrumb-remove {
        background: none;
        border: none;
        color: rgba(255, 255, 255, 0.4);
        cursor: pointer;
        padding: 0 0 0 0.15rem;
        font-size: 0.7rem;
        line-height: 1;
    }

    .breadcrumb-remove:hover {
        color: white;
    }

    .breadcrumb-add {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: 1.5rem;
        height: 1.5rem;
        background: rgba(255, 255, 255, 0.06);
        border: 1px dashed rgba(255, 255, 255, 0.2);
        border-radius: 4px;
        color: rgba(255, 255, 255, 0.5);
        font-size: 1rem;
        cursor: pointer;
    }

    .breadcrumb-add:hover {
        background: rgba(70, 130, 200, 0.2);
        border-color: rgba(70, 130, 200, 0.4);
        color: white;
    }

    .breadcrumb-clear {
        background: none;
        border: 1px solid rgba(255, 255, 255, 0.12);
        color: rgba(255, 255, 255, 0.4);
        padding: 0.2rem 0.5rem;
        border-radius: 4px;
        font-size: 0.65rem;
        cursor: pointer;
        margin-left: auto;
    }

    .breadcrumb-clear:hover {
        background: rgba(255, 255, 255, 0.08);
        color: white;
    }

    /* Picker overlay shared styles */
    .picker-overlay {
        position: fixed;
        inset: 0;
        background: rgba(0, 0, 0, 0.6);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 100;
    }

    .picker-panel {
        background: #1a1a2e;
        border: 1px solid rgba(255, 255, 255, 0.12);
        border-radius: 8px;
        min-width: 320px;
        max-width: 480px;
        max-height: 70vh;
        display: flex;
        flex-direction: column;
        box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
    }

    .picker-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0.75rem 1rem;
        border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    }

    .picker-title {
        margin: 0;
        font-size: 1rem;
        font-weight: 600;
        text-transform: capitalize;
    }

    .picker-close {
        background: none;
        border: none;
        color: rgba(255, 255, 255, 0.4);
        font-size: 1.1rem;
        cursor: pointer;
        padding: 0.2rem;
    }

    .picker-close:hover {
        color: white;
    }

    .picker-body {
        flex: 1;
        overflow-y: auto;
        padding: 0.5rem;
    }

    .picker-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        width: 100%;
        background: none;
        border: none;
        border-radius: 4px;
        padding: 0.5rem 0.75rem;
        color: rgba(255, 255, 255, 0.8);
        font-size: 0.85rem;
        cursor: pointer;
        text-align: left;
    }

    .picker-item:hover {
        background: rgba(70, 130, 200, 0.2);
    }
</style>
