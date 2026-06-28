<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { cfg, queries, env, activeTab, runOpts, isRunning } from '../stores';
  import type { QueryPayload, ResultPayload, DonePayload } from '../stores';
  import { EventsOn } from '../../wailsjs/runtime/runtime';
  import { StartRun, CancelRun, OpenRunFolder, ArchiveRun, ArchiveRunAuto, PruneRunDir } from '../../wailsjs/go/main/App';
  import type { archive } from '../../wailsjs/go/models';
  import Hint from '../components/Hint.svelte';
  import Select from '../components/Select.svelte';

  let running = false;
  // Mirror local run state into the shared store so App.svelte can lock the
  // other tabs while a run is active (prevents this view from unmounting).
  $: $isRunning = running;
  let total = 0;
  let current: QueryPayload | null = null;
  let result: ResultPayload | null = null;
  let logs: string[] = [];
  let done: DonePayload | null = null;
  let startError = '';
  let hold = 0;
  let holdTimer: any = null;
  let capturing = false;

  let archiving = false;
  let archiveResult: archive.Result | null = null;
  let archiveError = '';
  let pruned = false;

  // password prompt modal (explicit mode with no stored password)
  let pwModalOpen = false;
  let pwInput = '';
  let pwResolver: ((v: string | null) => void) | null = null;

  const unsub: Array<() => void> = [];

  onMount(() => {
    unsub.push(EventsOn('run:start', (p: any) => {
      running = true; total = p.total; current = null; result = null; logs = []; done = null;
      archiveResult = null; archiveError = ''; archiving = false; pruned = false;
    }));
    unsub.push(EventsOn('run:query', (p: QueryPayload) => { current = p; result = null; capturing = false; stopHold(); }));
    unsub.push(EventsOn('run:result', (p: ResultPayload) => {
      result = p;
      // The backend now settles + screenshots before the dwell; show that phase.
      capturing = p.status !== 'error';
      hold = 0;
    }));
    unsub.push(EventsOn('run:dwell', (p: any) => { capturing = false; startHold(p.seconds); }));
    unsub.push(EventsOn('run:log', (p: any) => { logs = [...logs, p.message]; }));
    unsub.push(EventsOn('run:done', (p: DonePayload) => {
      running = false; done = p; current = null; capturing = false; stopHold();
      if (p.error) { startError = p.error; return; }
      // Don't auto-archive an interrupted run — its evidence is incomplete.
      if (p.cancelled) { logs = [...logs, 'run cancelled — ZIP archive skipped']; return; }
      if (p.runDir && $runOpts?.zip) archive(p.runDir);
    }));
  });

  async function archive(runDir: string) {
    archiving = true; archiveError = ''; archiveResult = null; pruned = false;
    const excludeVideo = $runOpts?.excludeVideoFromZip ?? false;
    try {
      // ZIP password policy is a Settings concern (read from $cfg); whether to ZIP
      // at all and what to exclude/prune is per-run ($runOpts).
      const mode = $cfg?.zipPasswordMode ?? 'none';
      if (mode === 'auto') {
        archiveResult = await ArchiveRunAuto(runDir, excludeVideo);
      } else if (mode === 'explicit') {
        let pw = $cfg?.zipPassword ?? '';
        if (!pw) {
          const entered = await askPassword();
          if (!entered) { logs = [...logs, 'archiving skipped — no password provided']; return; }
          pw = entered;
        }
        archiveResult = await ArchiveRun(runDir, pw, excludeVideo);
      } else {
        archiveResult = await ArchiveRun(runDir, '', excludeVideo);
      }
      // Only after a confirmed archive, optionally drop the loose source files.
      if (archiveResult && $runOpts?.deleteSourcesAfterZip) {
        await PruneRunDir(runDir, excludeVideo);
        pruned = true;
      }
    } catch (e) {
      archiveError = String(e);
    } finally {
      archiving = false;
    }
  }

  function askPassword(): Promise<string | null> {
    pwInput = ''; pwModalOpen = true;
    return new Promise((res) => { pwResolver = res; });
  }
  function pwSubmit() { pwModalOpen = false; const r = pwResolver; pwResolver = null; r && r(pwInput || null); }
  function pwCancel() { pwModalOpen = false; const r = pwResolver; pwResolver = null; r && r(null); }

  onDestroy(() => { unsub.forEach((u) => u && u()); stopHold(); });

  function startHold(seconds?: number) {
    stopHold();
    hold = seconds ?? $cfg?.dwellSeconds ?? 0;
    if (hold > 0) {
      holdTimer = setInterval(() => {
        hold = Math.max(0, hold - 1);
        if (hold === 0) stopHold();
      }, 1000);
    }
  }
  function stopHold() { if (holdTimer) { clearInterval(holdTimer); holdTimer = null; } }

  $: enabled = $queries.filter((q) => q.enabled);
  // The Run page is the authority for the target connection this session; fall
  // back to the saved selection / first connection if the run id is missing.
  $: conn =
    $cfg?.connections.find((c) => c.id === $runOpts?.connectionId) ??
    $cfg?.connections.find((c) => c.id === $cfg?.selectedConnectionId) ??
    $cfg?.connections[0] ??
    null;
  $: canStart = !running && !!$env?.psqlFound && enabled.length > 0 && !!conn;

  async function start() {
    if (running) return; // guard a double-click before the first run:* event lands
    startError = '';
    running = true; // lock immediately; run:start will confirm, run:done releases
    try {
      // Per-run options come from $runOpts (not saved) — the Run page is the
      // authority for screenshots/video/connection; everything else is Settings.
      await StartRun(
        $runOpts?.screenshots ?? false,
        $runOpts?.video ?? false,
        $runOpts?.connectionId ?? conn?.id ?? '',
      );
    } catch (e) {
      startError = String(e);
      running = false; // start failed synchronously — release the lock
    }
  }
  async function cancel() { try { await CancelRun(); } catch {} }
  async function openFolder() { if (done?.runDir) { try { await OpenRunFolder(done.runDir); } catch {} } }

  // Run-tab controls mutate ONLY the ephemeral $runOpts store — never persisted.
  // They are seeded from saved settings at app start (see App.svelte).
  function toggleScreenshots() { if (!$runOpts) return; $runOpts.screenshots = !$runOpts.screenshots; $runOpts = $runOpts; }
  function toggleVideo() { if (!$runOpts || !$env?.ffmpegFound) return; $runOpts.video = !$runOpts.video; $runOpts = $runOpts; }
  function toggleZip() { if (!$runOpts) return; $runOpts.zip = !$runOpts.zip; $runOpts = $runOpts; }
  function toggleDelete() { if (!$runOpts) return; $runOpts.deleteSourcesAfterZip = !$runOpts.deleteSourcesAfterZip; $runOpts = $runOpts; }
  function toggleExcludeVideo() { if (!$runOpts) return; $runOpts.excludeVideoFromZip = !$runOpts.excludeVideoFromZip; $runOpts = $runOpts; }
</script>

<div class="wrap">
  <div class="bar">
    <div class="ctx">
      {#if $cfg && $runOpts}
        <span class="connsel">
          <Select compact bind:value={$runOpts.connectionId} disabled={running}
            options={$cfg.connections.map((cn) => ({ value: cn.id, label: cn.name }))} />
        </span>
      {/if}
      {#if $cfg?.enforceReadOnly}
        <span class="chip ro">read-only</span>
      {:else}
        <span class="chip rw" title="Read-only enforcement is OFF — queries can modify the database">read-write</span>
      {/if}
      <button class="toggle" class:on={$runOpts?.screenshots} on:click={toggleScreenshots} disabled={running}>
        Screenshots: {$runOpts?.screenshots ? 'on' : 'off'}
      </button>
      <span class="vidwrap" title={!$env?.ffmpegFound ? 'Video recording needs ffmpeg installed on this machine.' : ''}>
        <button class="toggle" class:on={$runOpts?.video && $env?.ffmpegFound}
          on:click={toggleVideo} disabled={running || !$env?.ffmpegFound}>
          Video: {$runOpts?.video && $env?.ffmpegFound ? 'on' : 'off'}
        </button>
      </span>
      <button class="toggle" class:on={$runOpts?.zip} on:click={toggleZip} disabled={running}>
        ZIP: {$runOpts?.zip ? 'on' : 'off'}
      </button>
      <button class="toggle" class:on={$runOpts?.zip && $runOpts?.deleteSourcesAfterZip}
        on:click={toggleDelete} disabled={running || !$runOpts?.zip}>
        Delete sources: {$runOpts?.zip && $runOpts?.deleteSourcesAfterZip ? 'on' : 'off'}
      </button>
      <button class="toggle" class:on={$runOpts?.zip && $runOpts?.excludeVideoFromZip}
        on:click={toggleExcludeVideo} disabled={running || !$runOpts?.zip}>
        Exclude video: {$runOpts?.zip && $runOpts?.excludeVideoFromZip ? 'on' : 'off'}
      </button>
    </div>
    <div class="spacer"></div>
    {#if running}
      <span class="progress">Query {current?.index ?? '–'} / {total}</span>
      <button class="danger" on:click={cancel}>Cancel run</button>
    {:else}
      <span class="qcount">{enabled.length} of {$queries.length} enabled</span>
      <button class="primary" on:click={start} disabled={!canStart}>▶ Start run</button>
    {/if}
  </div>

  {#if startError}<div class="error card">{startError}</div>{/if}

  {#if !running && !done}
    <div class="card placeholder">
      {#if !$env?.psqlFound}
        psql is not available — install PostgreSQL client tools.
      {:else if enabled.length === 0}
        No enabled queries. Add or enable queries on the Queries tab.
      {:else}
        Ready to run <strong>{enabled.length}</strong> quer{enabled.length === 1 ? 'y' : 'ies'} against
        <strong>{conn?.name}</strong>.
        <Hint text={`Each query runs read-only via psql, is saved as CSV with a SHA-256 checksum, shown on screen for ${$cfg?.dwellSeconds}s, and captured as a full-screen screenshot (with the OS clock as proof-of-time).`} />
      {/if}
    </div>
  {/if}

  {#if running && current}
    <div class="card stage">
      <div class="qhead">
        <span class="qidx">Query {current.index} / {total}</span>
        <span class="qname">{current.name}</span>
        {#if capturing}<span class="hold">capturing screenshot…</span>
        {:else if hold > 0}<span class="hold">holding for evidence… {hold}s</span>{/if}
      </div>
      <pre class="sql">{current.sql}</pre>

      {#if result}
        {#if result.status === 'error'}
          <div class="error">Query failed: {result.error}</div>
        {:else}
          <div class="checksum">
            <span class="lbl">SHA-256</span>
            <code>{result.sha256}</code>
            <span class="file">{result.resultFile}</span>
          </div>
          <div class="preview">
            <div class="prevhead">
              Result preview — showing {result.rows?.length ?? 0} of {result.rowCount} row{result.rowCount === 1 ? '' : 's'}
              <span class="muted"> · {result.durationMs} ms</span>
            </div>
            <div class="tablewrap scroll">
              <table>
                {#if result.header}
                  <thead><tr>{#each result.header as h}<th>{h}</th>{/each}</tr></thead>
                {/if}
                <tbody>
                  {#each result.rows ?? [] as r}
                    <tr>{#each r as cell}<td title={cell}>{cell}</td>{/each}</tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </div>
        {/if}
      {:else}
        <div class="muted running-msg">Running query…</div>
      {/if}
    </div>
  {/if}

  {#if done && !done.error}
    <div class="card summary">
      <h3>{done.cancelled ? 'Run cancelled' : 'Run complete'}</h3>
      <p>
        {#if done.cancelled}Run interrupted after {done.ok} succeeded{done.failed ? `, ${done.failed} failed` : ''}.
        {:else}{done.ok} succeeded{done.failed ? `, ${done.failed} failed` : ''}.{/if}
        {done.cancelled ? 'Partial evidence' : 'Evidence'} written to:
      </p>
      <code class="path">{done.runDir}</code>

      {#if $runOpts?.zip}
        <div class="archive">
          {#if archiving}
            <span class="muted">Creating ZIP archive…</span>
          {:else if archiveError}
            <span class="err">Archiving failed: {archiveError}</span>
          {:else if archiveResult}
            <div>ZIP archive: <code class="path">{archiveResult.zipPath}</code></div>
            {#if archiveResult.mode === 'auto'}
              <div style="margin-top:6px;">
                Generated password: <code class="pw">{archiveResult.password}</code>
                <span class="muted"> — saved to {archiveResult.pwdPath}</span>
              </div>
            {:else if archiveResult.encrypted}
              <div class="muted" style="margin-top:6px;">Encrypted (ZipCrypto) with your password.</div>
            {/if}
            {#if pruned}<div class="muted" style="margin-top:6px;">Source files removed — only the ZIP remains.</div>{/if}
          {/if}
        </div>
      {/if}

      <div class="row" style="margin-top:12px;">
        <button class="primary" on:click={openFolder}>Open evidence folder</button>
        <button class="ghost" on:click={() => ($activeTab = 'queries')}>Back to queries</button>
      </div>
    </div>
  {/if}

  {#if pwModalOpen}
    <div class="overlay">
      <div class="modal card">
        <h3>ZIP password</h3>
        <p class="muted">Enter a password to encrypt the archive, or cancel to skip archiving.</p>
        <input type="password" bind:value={pwInput} placeholder="password"
          on:keydown={(e) => e.key === 'Enter' && pwSubmit()} />
        <div class="row" style="margin-top:12px;">
          <div class="spacer"></div>
          <button class="ghost" on:click={pwCancel}>Skip</button>
          <button class="primary" on:click={pwSubmit} disabled={!pwInput}>Encrypt</button>
        </div>
      </div>
    </div>
  {/if}

  {#if logs.length}
    <div class="card log">
      <div class="muted" style="margin-bottom:6px;">Log</div>
      {#each logs as l}<div class="logline">{l}</div>{/each}
    </div>
  {/if}
</div>

<style>
  .wrap { height: 100%; padding: 16px; display: flex; flex-direction: column; gap: 14px; overflow: hidden; }
  .bar { display: flex; align-items: center; gap: 10px; }
  .ctx { display: flex; gap: 6px; flex-wrap: wrap; align-items: center; }
  .chip { background: var(--bg-3); border: 1px solid var(--border-strong); border-radius: 20px; padding: 3px 11px; font-size: 0.78rem; }
  .chip.ro { border-color: #3a5a3a; color: var(--ok); }
  .chip.rw { border-color: var(--err); color: var(--err); background: #ff6b6b1a; font-weight: 700; }
  .progress { font-size: 0.9rem; color: var(--muted); }
  .qcount { font-size: 0.85rem; color: var(--muted); }
  .connsel { display: inline-block; width: 200px; }
  .vidwrap { display: inline-flex; }
  .toggle { padding: 3px 11px; font-size: 0.78rem; border-radius: 20px; }
  .toggle.on { background: var(--accent-2); border-color: var(--accent-2); color: var(--on-accent); }
  .toggle.on:not(:disabled):hover { background: var(--accent-hover); border-color: var(--accent-hover); }
  .placeholder { padding: 32px; text-align: center; color: var(--muted); line-height: 1.6; }
  .stage { display: flex; flex-direction: column; gap: 14px; flex: 1 1 auto; min-height: 0; }
  .qhead { display: flex; align-items: baseline; gap: 12px; flex-wrap: wrap; }
  .qidx { color: var(--accent); font-weight: 700; }
  .qname { font-size: 1.25rem; font-weight: 700; }
  .hold { margin-left: auto; color: var(--warn); font-size: 0.85rem; }
  .sql {
    margin: 0; font-family: var(--mono); font-size: 1rem; line-height: 1.5;
    background: var(--code-bg); border: 1px solid var(--border-strong); border-radius: 8px;
    padding: 14px; white-space: pre-wrap; word-break: break-word; max-height: 220px; overflow: auto;
  }
  .checksum { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; background: var(--bg-2); border-radius: 8px; padding: 10px 14px; }
  .checksum .lbl { color: var(--accent); font-weight: 700; font-size: 0.85rem; }
  .checksum code { font-family: var(--mono); font-size: 0.95rem; word-break: break-all; }
  .checksum .file { color: var(--muted); font-family: var(--mono); font-size: 0.85rem; margin-left: auto; }
  .preview { flex: 1 1 auto; display: flex; flex-direction: column; min-height: 0; }
  .prevhead { font-size: 0.85rem; color: var(--muted); margin-bottom: 6px; }
  .tablewrap { border: 1px solid var(--border-strong); border-radius: 8px; flex: 1 1 auto; min-height: 0; }
  table { border-collapse: collapse; width: 100%; font-size: 0.85rem; }
  th, td { text-align: left; padding: 6px 10px; border-bottom: 1px solid var(--border-strong); white-space: nowrap; max-width: 320px; overflow: hidden; text-overflow: ellipsis; }
  thead th { position: sticky; top: 0; background: var(--bg-3); color: var(--text); }
  .error { color: var(--err); }
  .running-msg { padding: 20px 0; }
  .summary h3 { margin: 0 0 8px; }
  .path, code.path { font-family: var(--mono); font-size: 0.85rem; word-break: break-all; color: var(--muted); }
  .log { font-family: var(--mono); font-size: 0.8rem; max-height: 140px; overflow: auto; flex: 0 0 auto; }
  .logline { color: var(--warn); padding: 2px 0; }
  .archive { margin-top: 12px; padding: 10px 14px; background: var(--bg-2); border-radius: 8px; font-size: 0.9rem; }
  .pw { font-family: var(--mono); background: var(--bg-3); padding: 2px 8px; border-radius: 5px; }
  .overlay { position: fixed; inset: 0; background: #0008; display: flex; align-items: center; justify-content: center; z-index: 20; }
  .modal { width: 420px; max-width: 90vw; }
  .modal h3 { margin: 0 0 6px; }
</style>
