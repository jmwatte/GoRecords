<script>
  import { onMount } from 'svelte';
  import { initKeyboard, KEY, lastKey } from './lib/keyboard.js';
  import {
    viewMode, currentIndex, currentView, isVisualMode,
    isAlbumTracksView, toggleViewMode, openAlbum, closeAlbum,
    activeAlbumFolder
  } from './lib/stores.js';
  import { GetAlbumTracks, GetRandomAlbum } from '../wailsjs/go/main/App.js';
  import Settings from './lib/Settings.svelte';
  import FacetSidebar from './lib/FacetSidebar.svelte';
  import Toasts from './lib/Toasts.svelte';
  import { showToast } from './lib/toastStore.js';

  let cleanup;

  // Album list (fetched from backend — using mock for now until Phase 7)
  const totalAlbums = 50;
  const albums = Array.from({ length: totalAlbums }, (_, i) => ({
    albumFolder: `/mock/music/Album ${i + 1}`,
    album: `Album Title ${i + 1}`,
    albumArtist: `Artist ${String.fromCharCode(65 + (i % 26))}`,
    year: 1990 + (i % 35),
    genre: i % 3 === 0 ? 'Rock' : i % 3 === 1 ? 'Jazz' : 'Electronic',
    coverPath: '',
    trackCount: Math.floor(Math.random() * 15) + 5,
  }));

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
      tracks = await GetAlbumTracks(albumFolder) || [];
    } catch (err) {
      console.error('Failed to fetch tracks:', err);
      showToast('Failed to load album tracks', 'error');
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
    return () => {
      if (cleanup) cleanup();
    };
  });

  // Subscribe to keyboard events for navigation
  $: {
    if ($lastKey) {
      handleKey($lastKey);
    }
  }

  function handleKey(e) {
    // Global shortcuts (work in any view)
    if (e.key === KEY.REWIND) {
      handleRewind();
      e.stopPropagation();
      return;
    }

    if ($isAlbumTracksView) {
      // Navigation within album track list
      switch (e.key) {
        case KEY.ARROW_UP:
          $currentIndex = Math.max(0, $currentIndex - 1);
          e.stopPropagation();
          break;
        case KEY.ARROW_DOWN:
          $currentIndex = Math.min(tracks.length - 1, $currentIndex + 1);
          e.stopPropagation();
          break;
        case KEY.ENTER:
          handlePlaySelected();
          e.stopPropagation();
          break;
        case KEY.ESCAPE:
          handleCloseAlbum();
          e.stopPropagation();
          break;
      }
      return;
    }

    // Crate-level navigation
    switch (e.key) {
      case KEY.ARROW_LEFT:
        $currentIndex = Math.max(0, $currentIndex - 1);
        e.stopPropagation();
        break;
      case KEY.ARROW_RIGHT:
        $currentIndex = Math.min(totalAlbums - 1, $currentIndex + 1);
        e.stopPropagation();
        break;
      case KEY.ARROW_UP:
        $currentIndex = Math.max(0, $currentIndex - 1);
        e.stopPropagation();
        break;
      case KEY.ARROW_DOWN:
        $currentIndex = Math.min(totalAlbums - 1, $currentIndex + 1);
        e.stopPropagation();
        break;
      case KEY.ENTER:
        if (albums[$currentIndex]) {
          handleOpenAlbum(albums[$currentIndex].albumFolder);
        }
        e.stopPropagation();
        break;
      case KEY.LAYOUT_GRID:
        if (!$isVisualMode) toggleViewMode();
        e.stopPropagation();
        break;
      case KEY.LAYOUT_LIST:
        if ($isVisualMode) toggleViewMode();
        e.stopPropagation();
        break;
      case KEY.SETTINGS:
        $currentView = $currentView === 'settings' ? 'crate' : 'settings';
        e.stopPropagation();
        break;
      case KEY.RANDOM_ALBUM:
        handleRandomAlbum();
        e.stopPropagation();
        break;
    }
  }

  // Helper: format seconds to mm:ss
  function fmtDuration(sec) {
    if (!sec || sec <= 0) return '--:--';
    const m = Math.floor(sec / 60);
    const s = Math.floor(sec % 60);
    return `${m}:${s.toString().padStart(2, '0')}`;
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
  let currentTrackPath = '';
  let isPlaying = false;
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
    if (!trackPath) return '';
    return '/media/' + encodeURIComponent(trackPath);
  }

  // Play the track at the given index in the current tracks list
  function playTrack(idx) {
    if (!tracks[idx]) return;
    $currentIndex = idx;
    const track = tracks[idx];
    if (track.path && audioEl) {
      currentTrackPath = track.path;
      audioEl.src = audioSrc(track.path);
      audioEl.play().catch(err => {
        console.error('Playback failed:', err);
        showToast('Playback failed for this track', 'warn');
      });
      isPlaying = true;
      updateMediaSession(track);
    }
  }

  // Play selected track on Enter in album_tracks view
  function handlePlaySelected() {
    if ($isAlbumTracksView && tracks.length > 0 && $currentIndex >= 0 && $currentIndex < tracks.length) {
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
      audioEl.play().catch(err => console.error('Playback failed:', err));
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

  // Pick a random album and navigate to its track list
  async function handleRandomAlbum() {
    if ($currentView !== 'crate') return;
    try {
      const albumFolder = await GetRandomAlbum('');
      if (albumFolder) {
        await handleOpenAlbum(albumFolder);
        showToast('Playing random album', 'info', 2000);
      } else {
        showToast('No albums found', 'warn', 2000);
      }
    } catch (err) {
      console.error('Random album failed:', err);
      showToast('Failed to pick random album', 'error');
    }
  }

  // Media Session API: push metadata and action handlers to the OS media bar
  function updateMediaSession(track) {
    if (!('mediaSession' in navigator)) return;

    const albumInfo = albums.find(a => a.albumFolder === $activeAlbumFolder);

    navigator.mediaSession.metadata = new MediaMetadata({
      title: track.title || 'Unknown Track',
      artist: track.artist || track.albumArtist || 'Unknown Artist',
      album: track.album || albumInfo?.album || 'Unknown Album',
      artwork: track.coverPath
        ? [{ src: audioSrc(track.coverPath), sizes: '512x512', type: 'image/jpeg' }]
        : [],
    });

    navigator.mediaSession.setActionHandler('play', () => {
      audioEl?.play().catch(() => {});
    });
    navigator.mediaSession.setActionHandler('pause', () => {
      audioEl?.pause();
    });
    navigator.mediaSession.setActionHandler('previoustrack', () => {
      const prev = $currentIndex - 1;
      if (prev >= 0) playTrack(prev);
    });
    navigator.mediaSession.setActionHandler('nexttrack', () => {
      const next = $currentIndex + 1;
      if (next < tracks.length) playTrack(next);
    });
  }
</script>

<div class="app-shell">
  {#if $currentView === 'crate'}
    <div class="crate-layout">
      <FacetSidebar />
      <div class="crate-content">
        {#if $isVisualMode}
          <div class="visual-crate">
            <div class="album-art-frame">
              {#if albums[$currentIndex]?.coverPath}
                <img
                  class="album-art"
                  src={albums[$currentIndex].coverPath}
                  alt={albums[$currentIndex].album}
                />
              {:else}
                <div class="album-art-placeholder">
                  <span class="placeholder-text">{albums[$currentIndex]?.album?.[0] ?? '?'}</span>
                </div>
              {/if}
            </div>
            <div class="album-meta">
              <h1 class="album-title">{albums[$currentIndex]?.album ?? '—'}</h1>
              <h2 class="album-artist">{albums[$currentIndex]?.albumArtist ?? '—'}</h2>
              <span class="album-year">{albums[$currentIndex]?.year ?? ''}</span>
              <span class="sep">·</span>
              <span class="album-genre">{albums[$currentIndex]?.genre ?? ''}</span>
            </div>
            <div class="nav-hint">←  {($currentIndex + 1)} / {totalAlbums}  →</div>
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
                  on:click={() => handleOpenAlbum(album.albumFolder)}
                  on:dblclick={() => handleOpenAlbum(album.albumFolder)}
                >
                  <span class="col-artist">{album.albumArtist}</span>
                  <span class="col-album">{album.album}</span>
                  <span class="col-year">{album.year}</span>
                  <span class="col-genre">{album.genre}</span>
                  <span class="col-tracks">{album.trackCount}</span>
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    </div>
  {:else if $currentView === 'album_tracks'}
    <div class="album-tracks-view">
      <button class="back-btn" on:click={handleCloseAlbum}>← Back to Crate</button>
      <h2 class="tracks-album-title">
        {albums.find(a => a.albumFolder === $activeAlbumFolder)?.album ?? 'Album'}
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
        <div class="tracks-body">
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
                <span class="col-track">{track.trackNumber}</span>
                <span class="col-disc">{track.discNumber > 0 ? track.discNumber : ''}</span>
                <span class="col-title">{track.title || track.path?.split(/[\\/]/).pop()}</span>
                <span class="col-artist">{track.artist}</span>
                <span class="col-duration">{fmtDuration(track.duration)}</span>
              </div>
            {/each}
          {/each}
        </div>
      {/if}
    </div>
  {:else if $currentView === 'settings'}
    <Settings />
  {/if}

  <!-- Fixed Now Playing bar -->
  <div class="now-playing-bar">
    <div class="np-track-info">
      {#if tracks.length > 0 && tracks[$currentIndex]}
        <span class="np-title">{tracks[$currentIndex].title || 'Unknown Track'}</span>
        <span class="np-artist">{tracks[$currentIndex].artist || tracks[$currentIndex].albumArtist || 'Unknown Artist'}</span>
      {:else}
        <span class="np-title np-muted">No track selected</span>
        <span class="np-artist np-muted">Select a track to play</span>
      {/if}
    </div>

    <div class="np-controls">
      <button class="np-btn" on:click={prevTrack} title="Previous (p)">⏮</button>
      <button class="np-btn np-play-btn" on:click={handlePlayPause} title="Play/Pause (Space)">
        {isPlaying ? '⏸' : '▶'}
      </button>
      <button class="np-btn" on:click={nextTrack} title="Next (n)">⏭</button>
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
        {volume === 0 ? '🔇' : volume < 0.5 ? '🔉' : '🔊'}
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
    on:play={() => isPlaying = true}
    on:pause={() => isPlaying = false}
    on:timeupdate={onTimeUpdate}
    on:loadedmetadata={onLoadedMetadata}
  ></audio>
</div>

<style>
  .app-shell {
    flex: 1;
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
    justify-content: center;
    gap: 1.5rem;
    padding: 2rem;
    user-select: none;
  }

  .album-art-frame {
    width: min(60vh, 420px);
    height: min(60vh, 420px);
    border-radius: 8px;
    overflow: hidden;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
    transition: transform 0.15s ease;
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

  .col-artist { flex: 2; }
  .col-album  { flex: 3; }
  .col-year   { flex: 0 0 60px; }
  .col-genre  { flex: 1; }
  .col-tracks { flex: 0 0 60px; text-align: right; }

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

  .col-track    { flex: 0 0 40px; }
  .col-disc     { flex: 0 0 40px; }
  .col-title    { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .col-artist   { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: rgba(255, 255, 255, 0.5); }
  .col-duration { flex: 0 0 60px; text-align: right; }

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
    transition: color 0.1s, background 0.1s;
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
</style>
