<script lang="ts">
  // Modal "About" dialog. Opened from the (i) button in the header brand.
  // Links open in the user's default browser via the Wails runtime (the app
  // has no navigation of its own).
  import { env } from '../stores';
  import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';

  export let open = false;

  const DOCS_URL = 'https://michal-bartak.github.io/PgEvidence/';
  const GITHUB_URL = 'https://github.com/michal-bartak/PgEvidence';

  function close() {
    open = false;
  }
  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }
  function openUrl(url: string) {
    BrowserOpenURL(url);
  }
</script>

<svelte:window on:keydown={onKey} />

{#if open}
  <div class="overlay" on:click={close} role="presentation">
    <div class="dialog" on:click|stopPropagation role="dialog" aria-modal="true" aria-label="About">
      <button class="x" on:click={close} aria-label="Close">✕</button>

      <div class="head">
        <span class="dot"></span>
        <strong>{$env?.appName ?? 'PgEvidence'}</strong>
        {#if $env?.appVersion}<span class="ver">v{$env.appVersion}</span>{/if}
      </div>

      <p class="desc">
        A cross-platform desktop app for running ad-hoc PostgreSQL extracts for
        auditors — it produces reproducible, tamper-evident evidence that each
        query was run and what it returned.
      </p>

      <div class="links">
        <button on:click={() => openUrl(DOCS_URL)}>
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M2.5 2.5h6a2 2 0 012 2v9a1.5 1.5 0 00-1.5-1.5h-6.5z"/>
            <path d="M13.5 2.5h-3a2 2 0 00-2 2v9a1.5 1.5 0 011.5-1.5h3.5z"/>
          </svg>
          Documentation
        </button>
        <button on:click={() => openUrl(GITHUB_URL)}>
          <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
            <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
          </svg>
          GitHub
        </button>
      </div>

      <div class="section">
        <span class="lbl">Credits</span>
        <p>Created by Michal Bartak
          <br />
          <span class="ai">assisted by
            <a href={'#'} on:click|preventDefault={() => openUrl('https://claude.ai')}>Claude</a></span>
        </p>
      </div>

      <div class="section">
        <span class="lbl">License</span>
        <p>Released under the MIT License. © 2026 Michal Bartak.</p>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed; inset: 0; z-index: 200;
    background: #0008;
    display: flex; align-items: center; justify-content: center;
  }
  .dialog {
    position: relative;
    width: min(440px, calc(100vw - 48px));
    background: var(--panel); color: var(--text);
    border: 1px solid var(--border-strong); border-radius: 12px;
    padding: 22px 24px;
  }
  .x {
    position: absolute; top: 10px; right: 10px;
    width: 26px; height: 26px; padding: 0; line-height: 1;
    border-radius: 6px; background: transparent; border: 1px solid transparent;
    color: var(--muted);
  }
  .x:not(:disabled):hover { background: var(--bg-3); color: var(--text); }
  .head { display: flex; align-items: center; gap: 8px; font-size: 1.15rem; }
  .dot { width: 11px; height: 11px; border-radius: 50%; background: var(--accent); box-shadow: 0 0 8px var(--accent); }
  .ver { font-size: 0.72rem; color: var(--muted); background: var(--bg); border: 1px solid var(--border-strong); border-radius: 10px; padding: 1px 7px; }
  .desc { margin: 14px 0 0; font-size: 0.9rem; line-height: 1.55; color: var(--text); }
  .links { display: flex; gap: 10px; margin: 18px 0 4px; }
  .links button { flex: 1; display: inline-flex; align-items: center; justify-content: center; gap: 7px; }
  .links button svg { flex: 0 0 auto; }
  .section { margin-top: 16px; }
  .section .lbl { font-size: 0.72rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--muted); }
  .section p { margin: 4px 0 0; font-size: 0.88rem; line-height: 1.5; }
  .section a { color: var(--accent); }
  .ai { font-size: 0.78rem; color: var(--muted); }
</style>
