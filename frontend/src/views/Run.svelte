<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { cfg, queries, env, activeTab } from '../stores';
  import type { QueryPayload, ResultPayload, DonePayload } from '../stores';
  import { EventsOn } from '../../wailsjs/runtime/runtime';
  import { StartRun, CancelRun, OpenRunFolder, SaveConfig } from '../../wailsjs/go/main/App';

  let running = false;
  let total = 0;
  let current: QueryPayload | null = null;
  let result: ResultPayload | null = null;
  let logs: string[] = [];
  let done: DonePayload | null = null;
  let startError = '';
  let hold = 0;
  let holdTimer: any = null;
  let capturing = false;

  const unsub: Array<() => void> = [];

  onMount(() => {
    unsub.push(EventsOn('run:start', (p: any) => {
      running = true; total = p.total; current = null; result = null; logs = []; done = null;
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
      if (p.error) startError = p.error;
    }));
  });

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
  $: conn = $cfg?.connections.find((c) => c.id === $cfg?.selectedConnectionId) ?? null;
  $: canStart = !running && !!$env?.psqlFound && enabled.length > 0 && !!conn;

  async function start() {
    startError = '';
    try {
      // Persist the current (on-screen) settings first, so the run uses exactly
      // what's shown — StartRun reads the saved config on the backend.
      if ($cfg) await SaveConfig($cfg);
      await StartRun();
    } catch (e) { startError = String(e); }
  }
  async function cancel() { try { await CancelRun(); } catch {} }
  async function openFolder() { if (done?.runDir) { try { await OpenRunFolder(done.runDir); } catch {} } }
</script>

<div class="wrap">
  <div class="bar">
    <div class="ctx">
      <span class="chip">{conn ? conn.name : 'no connection'}</span>
      {#if $cfg?.enforceReadOnly}<span class="chip ro">read-only</span>{/if}
      {#if $cfg?.screenshots}<span class="chip">screenshots</span>{/if}
      {#if $cfg?.video && $env?.ffmpegFound}<span class="chip">video</span>{/if}
      <span class="chip">{enabled.length} quer{enabled.length === 1 ? 'y' : 'ies'}</span>
    </div>
    <div class="spacer"></div>
    {#if running}
      <span class="progress">Query {current?.index ?? '–'} / {total}</span>
      <button class="danger" on:click={cancel}>Cancel run</button>
    {:else}
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
        <strong>{conn?.name}</strong>. Each result is written as CSV with a SHA-256 checksum, shown here for
        {$cfg?.dwellSeconds}s, and captured as a full-screen screenshot.
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
      <h3>Run complete</h3>
      <p>
        {done.ok} succeeded{done.failed ? `, ${done.failed} failed` : ''}{done.cancelled ? ' (cancelled)' : ''}.
        Evidence written to:
      </p>
      <code class="path">{done.runDir}</code>
      <div class="row" style="margin-top:12px;">
        <button class="primary" on:click={openFolder}>Open evidence folder</button>
        <button class="ghost" on:click={() => ($activeTab = 'queries')}>Back to queries</button>
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
  .wrap { height: 100%; padding: 16px; display: flex; flex-direction: column; gap: 14px; overflow: auto; }
  .bar { display: flex; align-items: center; gap: 10px; }
  .ctx { display: flex; gap: 6px; flex-wrap: wrap; }
  .chip { background: var(--bg-3); border: 1px solid var(--border-strong); border-radius: 20px; padding: 3px 11px; font-size: 0.78rem; }
  .chip.ro { border-color: #3a5a3a; color: var(--ok); }
  .progress { font-size: 0.9rem; color: var(--muted); }
  .placeholder { padding: 32px; text-align: center; color: var(--muted); line-height: 1.6; }
  .stage { display: flex; flex-direction: column; gap: 14px; }
  .qhead { display: flex; align-items: baseline; gap: 12px; flex-wrap: wrap; }
  .qidx { color: var(--accent); font-weight: 700; }
  .qname { font-size: 1.25rem; font-weight: 700; }
  .hold { margin-left: auto; color: var(--warn); font-size: 0.85rem; }
  .sql {
    margin: 0; font-family: var(--mono); font-size: 1rem; line-height: 1.5;
    background: #0d1626; border: 1px solid var(--border-strong); border-radius: 8px;
    padding: 14px; white-space: pre-wrap; word-break: break-word; max-height: 220px; overflow: auto;
  }
  .checksum { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; background: var(--bg-2); border-radius: 8px; padding: 10px 14px; }
  .checksum .lbl { color: var(--accent); font-weight: 700; font-size: 0.85rem; }
  .checksum code { font-family: var(--mono); font-size: 0.95rem; word-break: break-all; }
  .checksum .file { color: var(--muted); font-family: var(--mono); font-size: 0.85rem; margin-left: auto; }
  .prevhead { font-size: 0.85rem; color: var(--muted); margin-bottom: 6px; }
  .tablewrap { border: 1px solid var(--border-strong); border-radius: 8px; max-height: 320px; }
  table { border-collapse: collapse; width: 100%; font-size: 0.85rem; }
  th, td { text-align: left; padding: 6px 10px; border-bottom: 1px solid var(--border-strong); white-space: nowrap; max-width: 320px; overflow: hidden; text-overflow: ellipsis; }
  thead th { position: sticky; top: 0; background: var(--bg-3); color: var(--text); }
  .error { color: var(--err); }
  .running-msg { padding: 20px 0; }
  .summary h3 { margin: 0 0 8px; }
  .path, code.path { font-family: var(--mono); font-size: 0.85rem; word-break: break-all; color: var(--muted); }
  .log { font-family: var(--mono); font-size: 0.8rem; }
  .logline { color: var(--warn); padding: 2px 0; }
</style>
