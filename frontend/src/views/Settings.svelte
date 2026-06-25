<script lang="ts">
  import { cfg, env } from '../stores';
  import { config } from '../../wailsjs/go/models';
  import {
    SaveConfig, SelectOutputDir, TestConnection,
    SetSessionPassword, HasSessionPassword,
  } from '../../wailsjs/go/main/App';

  let password = '';
  let pwStored = false;
  let testMsg = '';
  let testOk = false;
  let saveMsg = '';
  let busy = false;

  $: c = $cfg;
  $: connIdx = c ? c.connections.findIndex((x) => x.id === c.selectedConnectionId) : -1;
  $: conn = c && connIdx >= 0 ? c.connections[connIdx] : null;
  $: displays = $env?.numDisplays ?? 1;

  $: if (conn) refreshPw(conn.id);
  async function refreshPw(id: string) {
    try { pwStored = await HasSessionPassword(id); } catch {}
  }

  function addConnection() {
    if (!c) return;
    const id = (crypto as any).randomUUID ? crypto.randomUUID() : 'conn-' + Date.now();
    c.connections = [...c.connections, config.Connection.createFrom({
      id, name: 'New connection', host: 'localhost', port: 5432,
      dbName: 'postgres', user: 'postgres', sslMode: 'prefer',
    })];
    c.selectedConnectionId = id;
    $cfg = c;
  }

  function removeConnection() {
    if (!c || c.connections.length <= 1 || !conn) return;
    c.connections = c.connections.filter((x) => x.id !== conn!.id);
    c.selectedConnectionId = c.connections[0].id;
    $cfg = c;
  }

  async function browse() {
    try {
      const dir = await SelectOutputDir();
      if (dir && c) { c.outputDir = dir; $cfg = c; }
    } catch (e) { saveMsg = String(e); }
  }

  async function setPassword() {
    if (!conn) return;
    await SetSessionPassword(conn.id, password);
    password = '';
    await refreshPw(conn.id);
  }
  async function clearPassword() {
    if (!conn) return;
    await SetSessionPassword(conn.id, '');
    await refreshPw(conn.id);
  }

  async function save() {
    if (!c) return;
    busy = true; saveMsg = '';
    try { await SaveConfig(c); saveMsg = 'Saved.'; }
    catch (e) { saveMsg = String(e); }
    busy = false;
    setTimeout(() => (saveMsg = ''), 2500);
  }

  async function test() {
    if (!conn) return;
    busy = true; testMsg = 'Testing…'; testOk = false;
    try {
      await SaveConfig(c!); // persist so the backend tests the current values
      await TestConnection(conn.id);
      testOk = true; testMsg = 'Connection OK.';
    } catch (e) { testOk = false; testMsg = String(e); }
    busy = false;
  }
</script>

{#if c}
<div class="wrap scroll">
  <div class="savebar">
    <button class="primary" on:click={save} disabled={busy}>Save settings</button>
    {#if saveMsg}<span class="muted" style="margin-left:4px;">{saveMsg}</span>{/if}
    <span class="muted" style="margin-left:auto; font-size:0.8rem;">Starting a run also saves the current settings.</span>
  </div>

  <div class="content">
  <div class="grid">
    <div class="card">
      <h3>Database connection</h3>
      <div class="row" style="align-items:flex-end;">
        <div class="col">
          <label for="connsel">Active connection</label>
          <select id="connsel" bind:value={c.selectedConnectionId}>
            {#each c.connections as x}<option value={x.id}>{x.name}</option>{/each}
          </select>
        </div>
        <button class="ghost" on:click={addConnection}>+ Add</button>
        <button class="danger" on:click={removeConnection} disabled={c.connections.length <= 1}>Remove</button>
      </div>

      {#if conn}
        <div style="margin-top:12px;">
          <label for="cname">Name</label>
          <input id="cname" bind:value={conn.name} />
          <div class="row" style="margin-top:10px;">
            <div class="col"><label for="chost">Host</label><input id="chost" bind:value={conn.host} /></div>
            <div style="width:110px;"><label for="cport">Port</label><input id="cport" type="number" bind:value={conn.port} /></div>
          </div>
          <div class="row" style="margin-top:10px;">
            <div class="col"><label for="cdb">Database</label><input id="cdb" bind:value={conn.dbName} /></div>
            <div class="col"><label for="cuser">User</label><input id="cuser" bind:value={conn.user} /></div>
          </div>
          <label for="cssl" style="margin-top:10px;">SSL mode</label>
          <select id="cssl" bind:value={conn.sslMode}>
            {#each ['disable','allow','prefer','require','verify-ca','verify-full'] as m}
              <option value={m}>{m}</option>
            {/each}
          </select>

          <div class="pw card" style="margin-top:14px; background:var(--bg-2);">
            <!-- svelte-ignore a11y-label-has-associated-control -->
            <label>Session password (held in memory only — never written to disk)</label>
            <div class="row">
              <input type="password" bind:value={password} placeholder={pwStored ? '•••••••• (set)' : "leave blank to use ~/.pgpass"} />
              <button on:click={setPassword} disabled={!password}>Set</button>
              <button class="ghost" on:click={clearPassword} disabled={!pwStored}>Clear</button>
            </div>
            <div class="row" style="margin-top:10px; align-items:center;">
              <button on:click={test} disabled={busy}>Test connection</button>
              {#if testMsg}<span class:ok={testOk} class:err={!testOk} class="testmsg">{testMsg}</span>{/if}
            </div>
          </div>
        </div>
      {/if}
    </div>

    <div class="card">
      <h3>Run & evidence</h3>
      <label for="outdir">Output folder</label>
      <div class="row">
        <input id="outdir" bind:value={c.outputDir} />
        <button class="ghost" on:click={browse}>Browse…</button>
      </div>

      <div class="row" style="margin-top:12px;">
        <div class="col">
          <label for="dwell">Dwell time per query (seconds)</label>
          <input id="dwell" type="number" min="0" bind:value={c.dwellSeconds} />
        </div>
        <div class="col">
          <label for="prev">Preview rows on screen</label>
          <input id="prev" type="number" min="0" bind:value={c.previewRows} />
        </div>
      </div>

      <label for="mon" style="margin-top:12px;">Monitor to capture</label>
      <select id="mon" bind:value={c.monitorIndex}>
        {#each Array(Math.max(displays, 1)) as _, i}<option value={i}>Display {i}</option>{/each}
      </select>

      <div class="toggles">
        <label class="toggle"><input type="checkbox" bind:checked={c.enforceReadOnly} /> Enforce read-only transactions</label>
        <label class="toggle"><input type="checkbox" bind:checked={c.screenshots} /> Full-screen screenshot per query</label>
        <label class="toggle" class:disabled={!$env?.ffmpegFound}>
          <input type="checkbox" bind:checked={c.video} disabled={!$env?.ffmpegFound} />
          Record video (ffmpeg){#if !$env?.ffmpegFound}<span class="muted"> — ffmpeg not installed</span>{/if}
        </label>
        <label class="toggle"><input type="checkbox" bind:checked={c.stopOnError} /> Stop run on first error</label>
      </div>
    </div>

    <div class="card env">
      <h3>Environment</h3>
      <table>
        <tr><td>App version</td><td>{$env?.appVersion}</td></tr>
        <tr><td>psql</td><td>{$env?.psqlFound ? $env.psqlVersion : 'not found'}</td></tr>
        <tr><td>psql path</td><td class="mono">{$env?.psqlPath || '—'}</td></tr>
        <tr><td>ffmpeg</td><td>{$env?.ffmpegFound ? 'available' : 'not installed (video disabled)'}</td></tr>
        <tr><td>Displays</td><td>{$env?.numDisplays}</td></tr>
        <tr><td>Config dir</td><td class="mono">{$env?.configDir}</td></tr>
      </table>
    </div>
  </div>

  </div>
</div>
{/if}

<style>
  .wrap { height: 100%; padding: 0; }
  .content { padding: 16px; }
  .savebar {
    position: sticky; top: 0; z-index: 5;
    display: flex; align-items: center; gap: 12px;
    padding: 12px 16px; background: var(--bg-2);
    border-bottom: 1px solid var(--border-strong);
  }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; align-items: start; }
  .env { grid-column: 1 / -1; }
  h3 { margin: 0 0 12px; }
  .toggles { display: flex; flex-direction: column; gap: 8px; margin-top: 14px; }
  .toggle { display: flex; align-items: center; gap: 8px; margin: 0; color: var(--text); font-size: 0.9rem; }
  .toggle input { width: auto; }
  .toggle.disabled { opacity: 0.55; }
  .testmsg { font-size: 0.85rem; }
  .testmsg.ok { color: var(--ok); }
  .testmsg.err { color: var(--err); }
  table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
  td { padding: 4px 8px; border-bottom: 1px solid var(--border-strong); vertical-align: top; }
  td:first-child { color: var(--muted); width: 110px; }
</style>
