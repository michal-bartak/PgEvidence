<script lang="ts">
  import { onMount } from 'svelte';
  import { activeTab, cfg, queries, env } from './stores';
  import { GetConfig, ListQueries, DetectEnvironment, RequestScreenAccess } from '../wailsjs/go/main/App';
  import { applyTheme } from './theme';
  import Queries from './views/Queries.svelte';
  import Run from './views/Run.svelte';
  import Settings from './views/Settings.svelte';

  let loaded = false;
  let loadError = '';

  onMount(async () => {
    try {
      $env = await DetectEnvironment();
      $cfg = await GetConfig();
      applyTheme($cfg.theme || 'system');
      $queries = await ListQueries();
      loaded = true;
    } catch (e) {
      loadError = String(e);
    }
  });

  const tabs: { id: 'queries' | 'run' | 'settings'; label: string }[] = [
    { id: 'queries', label: 'Queries' },
    { id: 'run', label: 'Run' },
    { id: 'settings', label: 'Settings' },
  ];

  let screenMsg = '';
  async function grantScreen() {
    await RequestScreenAccess();
    screenMsg = `If a system prompt appeared, enable ${$env?.appName ?? 'the app'}, then quit and reopen the app for it to take effect.`;
  }
</script>

<div class="shell">
  <header>
    <div class="brand">
      <span class="dot"></span>
      <strong>{$env?.appName ?? 'PgEvidence'}</strong>
      {#if $env?.appVersion}<span class="ver">v{$env.appVersion}</span>{/if}
      <span class="muted">— reproducible, checksummed DB extracts</span>
    </div>
    <nav>
      {#each tabs as t}
        <button class="tab" class:active={$activeTab === t.id} on:click={() => ($activeTab = t.id)}>
          {t.label}
        </button>
      {/each}
    </nav>
  </header>

  {#if $env && !$env.psqlFound}
    <div class="banner err">
      <strong>psql not found on PATH.</strong> Install PostgreSQL client tools to run extracts.
    </div>
  {/if}

  {#if $env && !$env.screenAccess}
    <div class="banner warn">
      <strong>macOS may not have granted Screen Recording.</strong>
      The app still tries to capture; if screenshots fail, grant it here.
      Ad-hoc-signed builds can need re-granting after each rebuild.
      <button class="ghost mini" on:click={grantScreen}>Grant permission…</button>
      {#if screenMsg}<span class="note">{screenMsg}</span>{/if}
    </div>
  {/if}

  <main>
    {#if loadError}
      <div class="card" style="margin:24px;">Failed to load: {loadError}</div>
    {:else if !loaded}
      <div class="muted" style="padding:24px;">Loading…</div>
    {:else if $activeTab === 'queries'}
      <Queries />
    {:else if $activeTab === 'run'}
      <Run />
    {:else}
      <Settings />
    {/if}
  </main>
</div>

<style>
  .shell { display: flex; flex-direction: column; height: 100%; }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 18px;
    background: var(--bg-2);
    border-bottom: 1px solid var(--border-strong);
    flex: 0 0 auto;
  }
  .brand { display: flex; align-items: center; gap: 8px; font-size: 1rem; }
  .dot { width: 10px; height: 10px; border-radius: 50%; background: var(--accent); box-shadow: 0 0 8px var(--accent); }
  .ver { font-size: 0.72rem; color: var(--muted); background: var(--bg); border: 1px solid var(--border-strong); border-radius: 10px; padding: 1px 7px; }
  nav { display: flex; gap: 6px; }
  .tab { background: transparent; }
  .tab.active { background: var(--accent-2); border-color: var(--accent-2); color: var(--on-accent); }
  .banner { padding: 8px 18px; font-size: 0.85rem; flex: 0 0 auto; }
  .banner.err { background: #3a1f1f; color: #ffc2c2; border-bottom: 1px solid #6b3030; }
  .banner.warn { background: #3a341c; color: #ffe6a3; border-bottom: 1px solid #6b5a2a; display: flex; align-items: center; gap: 10px; }
  .banner .mini { padding: 3px 10px; font-size: 0.8rem; }
  .banner .note { color: #d8c79a; font-size: 0.8rem; }
  main { flex: 1 1 auto; min-height: 0; overflow: hidden; }
</style>
