<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { activeTab, cfg, queries, env, runOpts, savedTick, isRunning } from './stores';
  import { GetConfig, ListQueries, DetectEnvironment, GrantScreenAccess, SaveWindowSize } from '../wailsjs/go/main/App';
  import { WindowGetSize } from '../wailsjs/runtime/runtime';
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
      // Seed the per-run options from saved settings (load-time sync only; the
      // Run page edits these for the session without writing back to config).
      $runOpts = {
        screenshots: $cfg.screenshots,
        video: $cfg.video,
        zip: $cfg.zip,
        deleteSourcesAfterZip: $cfg.deleteSourcesAfterZip,
        excludeVideoFromZip: $cfg.excludeVideoFromZip,
        connectionId: $cfg.selectedConnectionId,
      };
      $queries = await ListQueries();
      loaded = true;
    } catch (e) {
      loadError = String(e);
    }
  });

  // Persist the window size (debounced) so it's restored next launch. Save the
  // Wails OS window size — NOT window.innerWidth (the viewport excludes the OS
  // chrome, which would make the window shrink a little on every restart).
  let resizeTimer: ReturnType<typeof setTimeout>;
  function onResize() {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => {
      WindowGetSize().then(({ w, h }) => SaveWindowSize(w, h)).catch(() => {});
    }, 500);
  }
  onMount(() => window.addEventListener('resize', onResize));
  onDestroy(() => { clearTimeout(resizeTimer); window.removeEventListener('resize', onResize); });

  // Flash the Settings tab green ("Saved") whenever a settings auto-save lands.
  // Two phases: 'hold' keeps it solid green with the "Saved" label for a beat,
  // then 'fade' restores the label and eases the color back to the active color.
  const HOLD_MS = 1500;
  const FADE_MS = 700;
  let flashState: '' | 'hold' | 'fade' = '';
  let holdTimer: ReturnType<typeof setTimeout>;
  let fadeTimer: ReturnType<typeof setTimeout>;
  let savedSeen = false;
  savedTick.subscribe(() => {
    if (!savedSeen) { savedSeen = true; return; } // ignore the initial value
    clearTimeout(holdTimer);
    clearTimeout(fadeTimer);
    flashState = 'hold';
    holdTimer = setTimeout(() => {
      flashState = 'fade';
      fadeTimer = setTimeout(() => (flashState = ''), FADE_MS);
    }, HOLD_MS);
  });

  const tabs: { id: 'queries' | 'run' | 'settings'; label: string }[] = [
    { id: 'queries', label: 'Queries' },
    { id: 'run', label: 'Run' },
    { id: 'settings', label: 'Settings' },
  ];

  async function grantScreen() {
    // Backend decides: first time shows the system prompt (which itself offers
    // "Open System Settings"); afterwards it opens the Settings pane. Never both.
    await GrantScreenAccess();
  }
  // Temporary, non-persisted dismissal — restored on next app launch.
  let screenBannerHidden = false;
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
        <button
          class="tab"
          class:active={$activeTab === t.id}
          class:flash-hold={t.id === 'settings' && flashState === 'hold'}
          class:flash-fade={t.id === 'settings' && flashState === 'fade'}
          on:click={() => ($activeTab = t.id)}
          disabled={$isRunning && t.id !== $activeTab}
          title={$isRunning && t.id !== $activeTab ? 'Disabled while a run is in progress' : ''}
        >
          <!-- Sizer reserves the natural label width so swapping in "Saved"
               (shorter) doesn't shrink the button. -->
          <span class="sizer" aria-hidden="true">{t.label}</span>
          <span class="lbl">{t.id === 'settings' && flashState === 'hold' ? 'Saved' : t.label}</span>
        </button>
      {/each}
    </nav>
  </header>

  {#if $env && !$env.psqlFound}
    <div class="banner err">
      <strong>psql not found on PATH.</strong> Install PostgreSQL client tools to run extracts.
    </div>
  {/if}

  {#if $env && !$env.screenAccess && !screenBannerHidden}
    <div class="banner warn">
      <strong>macOS may not have granted Screen Recording.</strong>
      <button class="bbtn" on:click={grantScreen}>Grant permission…</button>
      <button class="bclose" title="Hide until next launch" aria-label="Hide"
        on:click={() => (screenBannerHidden = true)}>✕</button>
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
  .tab { background: transparent; position: relative; }
  .tab.active { background: var(--accent-2); border-color: var(--accent-2); color: var(--on-accent); }
  /* Keep the label centered over a fixed-width sizer so the button never resizes. */
  .tab .sizer { visibility: hidden; }
  .tab .lbl { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; }
  /* Settings save feedback: solid green during the hold, then ease back to the
     active-tab color while the label flips back to "Settings". */
  .tab.flash-hold { background: var(--ok); border-color: var(--ok); color: var(--on-accent); }
  .tab.flash-fade { color: var(--on-accent); animation: savedfade 0.7s ease-out forwards; }
  @keyframes savedfade {
    0%   { background: var(--ok); border-color: var(--ok); }
    100% { background: var(--accent-2); border-color: var(--accent-2); }
  }
  .banner { padding: 8px 18px; font-size: 0.85rem; flex: 0 0 auto; }
  .banner.err { background: #3a1f1f; color: #ffc2c2; border-bottom: 1px solid #6b3030; }
  .banner.warn { background: #3a341c; color: #ffe6a3; border-bottom: 1px solid #6b5a2a; display: flex; align-items: center; gap: 10px; }
  /* Banners are always dark, in both themes — style the button light-on-dark
     explicitly so it isn't invisible in light mode. */
  .banner .bbtn { padding: 3px 10px; font-size: 0.8rem; border-radius: 6px;
    background: transparent; border: 1px solid #b9a85f; color: #ffe6a3; }
  .banner .bbtn:not(:disabled):hover { background: #ffffff22; border-color: #ffe6a3; color: #fff; }
  .banner .bclose {
    margin-left: auto; padding: 2px 8px; font-size: 0.8rem; line-height: 1;
    border-radius: 6px; background: transparent; border: 1px solid transparent; color: #ffe6a3;
  }
  .banner .bclose:not(:disabled):hover { background: #ffffff22; border-color: #ffe6a3; color: #fff; }
  main { flex: 1 1 auto; min-height: 0; overflow: hidden; }
</style>
