<script>
  import { toasts, dismissToast } from './toastStore.js';
</script>

{#each $toasts as toast (toast.id)}
  <div
    class="toast toast-{toast.type}"
    role="alert"
    on:click={() => dismissToast(toast.id)}
  >
    <span class="toast-icon">
      {#if toast.type === 'error'}✕
      {:else if toast.type === 'warn'}⚠
      {:else}ℹ
      {/if}
    </span>
    <span class="toast-message">{toast.message}</span>
  </div>
{/each}

<style>
  .toast {
    position: fixed;
    bottom: 72px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 1rem;
    border-radius: 6px;
    font-size: 0.85rem;
    color: white;
    cursor: pointer;
    z-index: 9999;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
    animation: toast-in 0.25s ease-out;
    max-width: 80vw;
    pointer-events: auto;
  }

  .toast-info {
    background: rgba(40, 80, 160, 0.9);
    border: 1px solid rgba(70, 130, 200, 0.5);
  }

  .toast-warn {
    background: rgba(160, 120, 40, 0.9);
    border: 1px solid rgba(200, 160, 60, 0.5);
  }

  .toast-error {
    background: rgba(160, 40, 40, 0.9);
    border: 1px solid rgba(200, 60, 60, 0.5);
  }

  .toast-icon {
    flex-shrink: 0;
    font-size: 0.9rem;
    opacity: 0.8;
  }

  .toast-message {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  @keyframes toast-in {
    from {
      opacity: 0;
      transform: translateX(-50%) translateY(8px);
    }
    to {
      opacity: 1;
      transform: translateX(-50%) translateY(0);
    }
  }
</style>
