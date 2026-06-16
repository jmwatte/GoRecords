<script>
  import { musicRoot, scanProgress, currentView } from './stores.js';
  import { ScanMusic, PickFolder } from '../../wailsjs/go/main/App.js';
  import { showToast } from './toastStore.js';

  let loading = false;

  async function chooseFolder() {
    try {
      const dir = await PickFolder();
      if (dir) {
        $musicRoot = dir;
      }
    } catch (err) {
      console.error('Failed to open directory dialog:', err);
      showToast('Could not open folder picker', 'warn');
    }
  }

  async function rescanLibrary() {
    if (!$musicRoot) return;
    loading = true;
    $scanProgress = 0;
    try {
      await ScanMusic($musicRoot);
      $scanProgress = 100;
    } catch (err) {
      console.error('Scan failed:', err);
      showToast('Library scan failed: ' + (err.message || err), 'error');
      $scanProgress = -1;
    } finally {
      loading = false;
      // Return to crate after scan
      $currentView = 'crate';
      // Reset progress after a brief moment so the user sees 100%
      setTimeout(() => { $scanProgress = -1; }, 1500);
    }
  }

  function goBack() {
    $currentView = 'crate';
  }
</script>

<div class="settings-view">
  <button class="back-btn" on:click={goBack}>← Back to Library</button>
  <h2 class="settings-title">Settings</h2>

  <div class="setting-group">
    <label class="setting-label">Music Library Folder</label>
    <div class="folder-row">
      <input
        class="folder-input"
        type="text"
        readonly
        bind:value={$musicRoot}
        placeholder="No folder selected…"
      />
      <button class="btn" on:click={chooseFolder}>Choose…</button>
    </div>
  </div>

  <div class="setting-group">
    <button
      class="btn btn-primary"
      disabled={!$musicRoot || loading}
      on:click={rescanLibrary}
    >
      {loading ? 'Scanning…' : 'Rescan Library'}
    </button>
    <p class="hint">Scans the folder for audio files, extracts metadata, and updates the database.</p>
  </div>

  {#if $scanProgress >= 0}
    <div class="progress-bar-wrap">
      <div class="progress-bar" style="width: {$scanProgress}%"></div>
      <span class="progress-label">{$scanProgress}%</span>
    </div>
  {/if}
</div>

<style>
  .settings-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    padding: 1.5rem;
    overflow-y: auto;
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
    margin-bottom: 1rem;
    font-size: 0.85rem;
  }

  .back-btn:hover {
    background: rgba(255, 255, 255, 0.1);
  }

  .settings-title {
    margin: 0 0 1.5rem;
    font-size: 1.4rem;
    font-weight: 600;
  }

  .setting-group {
    margin-bottom: 1.5rem;
  }

  .setting-label {
    display: block;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: rgba(255, 255, 255, 0.5);
    margin-bottom: 0.5rem;
  }

  .folder-row {
    display: flex;
    gap: 0.5rem;
  }

  .folder-input {
    flex: 1;
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 4px;
    padding: 0.45rem 0.6rem;
    color: white;
    font-size: 0.85rem;
    cursor: default;
  }

  .folder-input::placeholder {
    color: rgba(255, 255, 255, 0.3);
  }

  .btn {
    background: rgba(255, 255, 255, 0.1);
    border: 1px solid rgba(255, 255, 255, 0.2);
    color: white;
    padding: 0.45rem 1rem;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
    white-space: nowrap;
  }

  .btn:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.18);
  }

  .btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .btn-primary {
    background: rgba(70, 130, 200, 0.6);
    border-color: rgba(70, 130, 200, 0.8);
  }

  .btn-primary:hover:not(:disabled) {
    background: rgba(70, 130, 200, 0.8);
  }

  .hint {
    margin: 0.5rem 0 0;
    font-size: 0.8rem;
    color: rgba(255, 255, 255, 0.35);
  }

  .progress-bar-wrap {
    margin-top: 0.5rem;
    height: 20px;
    background: rgba(255, 255, 255, 0.08);
    border-radius: 10px;
    overflow: hidden;
    position: relative;
  }

  .progress-bar {
    height: 100%;
    background: linear-gradient(90deg, rgba(70, 130, 200, 0.6), rgba(100, 180, 255, 0.8));
    border-radius: 10px;
    transition: width 0.3s ease;
  }

  .progress-label {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.75rem;
    font-weight: 600;
    color: white;
  }
</style>
