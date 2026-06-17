<script>
  import { activeFilters, toggleFilter, clearFilters, facetData } from './stores.js';

  // Define which facet fields to show and their display labels
  const FACET_FIELDS = [
    { field: 'genre', label: 'Genre' },
    { field: 'year', label: 'Year' },
    { field: 'artist', label: 'Artist' },
  ];

  // Check if a value is currently active (filter stack uses { category, value })
  function isActive(field, value) {
    return $activeFilters.some((f) => f.category === field && f.value === value);
  }

  function handleToggle(field, value) {
    toggleFilter(field, value);
  }
</script>

<div class="sidebar">
  <div class="sidebar-header">
    <h3 class="sidebar-title">Filters</h3>
    {#if $activeFilters.length > 0}
      <button class="clear-btn" on:click={clearFilters}>
        Clear ({$activeFilters.length})
      </button>
    {/if}
  </div>

  <div class="sidebar-body">
    {#each FACET_FIELDS as ff}
      <div class="facet-group">
        <h4 class="facet-label">{ff.label}</h4>
        <div class="facet-chips">
          {#if $facetData[ff.field]?.length > 0}
            {#each $facetData[ff.field] as item}
              <button
                class="chip"
                class:active={isActive(ff.field, item.value)}
                on:click={() => handleToggle(ff.field, item.value)}
                title="{item.value} ({item.count})"
              >
                <span class="chip-value">{item.value}</span>
                <span class="chip-count">{item.count}</span>
              </button>
            {/each}
          {:else}
            <span class="empty-hint">No data</span>
          {/if}
        </div>
      </div>
    {/each}
  </div>
</div>

<style>
  .sidebar {
    width: 220px;
    min-width: 220px;
    display: flex;
    flex-direction: column;
    border-right: 1px solid rgba(255, 255, 255, 0.08);
    background: rgba(255, 255, 255, 0.02);
    user-select: none;
  }

  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 0.75rem 0.5rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  }

  .sidebar-title {
    margin: 0;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: rgba(255, 255, 255, 0.4);
    font-weight: 600;
  }

  .clear-btn {
    background: none;
    border: 1px solid rgba(255, 255, 255, 0.15);
    color: rgba(255, 255, 255, 0.5);
    padding: 0.15rem 0.4rem;
    border-radius: 3px;
    font-size: 0.65rem;
    cursor: pointer;
  }

  .clear-btn:hover {
    background: rgba(255, 255, 255, 0.1);
    color: white;
  }

  .sidebar-body {
    flex: 1;
    overflow-y: auto;
    padding: 0.5rem 0.75rem;
  }

  .facet-group {
    margin-bottom: 1rem;
  }

  .facet-label {
    margin: 0 0 0.4rem;
    font-size: 0.65rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: rgba(255, 255, 255, 0.35);
    font-weight: 600;
  }

  .facet-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
  }

  .chip {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 4px;
    padding: 0.2rem 0.4rem;
    cursor: pointer;
    font-size: 0.75rem;
    color: rgba(255, 255, 255, 0.7);
    transition: all 0.1s;
    max-width: 100%;
  }

  .chip:hover {
    background: rgba(255, 255, 255, 0.12);
    border-color: rgba(255, 255, 255, 0.2);
  }

  .chip.active {
    background: rgba(70, 130, 200, 0.25);
    border-color: rgba(70, 130, 200, 0.6);
    color: white;
  }

  .chip-value {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chip-count {
    font-size: 0.65rem;
    color: rgba(255, 255, 255, 0.35);
    flex-shrink: 0;
  }

  .chip.active .chip-count {
    color: rgba(255, 255, 255, 0.6);
  }

  .empty-hint {
    font-size: 0.7rem;
    color: rgba(255, 255, 255, 0.2);
    font-style: italic;
  }
</style>
