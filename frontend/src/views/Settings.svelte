<script lang="ts">
  import { cfg, env, savedTick } from '../stores';
  import { config } from '../../wailsjs/go/models';
  import {
    SaveConfig, SelectOutputDir, SelectFile, TestConnection, DetectEnvironment,
    SetSessionPassword, HasSessionPassword, UpdateTheme,
  } from '../../wailsjs/go/main/App';
  import { applyTheme } from '../theme';
  import Hint from '../components/Hint.svelte';
  import Select from '../components/Select.svelte';
  import Stepper from '../components/Stepper.svelte';

  const sslModes = ['disable', 'allow', 'prefer', 'require', 'verify-ca', 'verify-full']
    .map((m) => ({ value: m, label: m }));
  const zipModes = [
    { value: 'none', label: 'None — plain ZIP' },
    { value: 'explicit', label: 'Explicit password' },
    { value: 'auto', label: 'Auto-generated password' },
  ];

  let password = '';
  let pwStored = false;
  let testMsg = '';
  let testStatus: '' | 'testing' | 'ok' | 'error' = '';
  let busy = false;
  let saveTimer: any;
  let prevConnId = '';

  $: c = $cfg;
  $: connIdx = c ? c.connections.findIndex((x) => x.id === c.selectedConnectionId) : -1;
  $: conn = c && connIdx >= 0 ? c.connections[connIdx] : null;

  // Reset/refresh the password field only when the selected connection changes,
  // so unrelated config edits (theme, etc.) don't clear what the user typed.
  $: if (conn && conn.id !== prevConnId) { prevConnId = conn.id; password = ''; refreshPw(conn.id); }
  async function refreshPw(id: string) {
    try { pwStored = await HasSessionPassword(id); } catch {}
  }

  // Debounced auto-save: every field change persists the config (no Save button).
  function autoSave() {
    if (!c) return;
    clearTimeout(saveTimer);
    saveTimer = setTimeout(async () => {
      try { await SaveConfig(c!); $savedTick++; } catch {}
    }, 400);
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
    autoSave();
  }

  function removeConnection() {
    if (!c || c.connections.length <= 1 || !conn) return;
    c.connections = c.connections.filter((x) => x.id !== conn!.id);
    c.selectedConnectionId = c.connections[0].id;
    $cfg = c;
    autoSave();
  }

  async function browse() {
    try {
      const dir = await SelectOutputDir();
      if (dir && c) { c.outputDir = dir; $cfg = c; autoSave(); }
    } catch {}
  }

  // Apply the session password on edit (blur/Enter); empty clears it.
  async function applyPassword() {
    if (!conn) return;
    await SetSessionPassword(conn.id, password);
    await refreshPw(conn.id);
  }

  async function browseTool(which: 'psql' | 'ffmpeg') {
    if (!c) return;
    try {
      const p = await SelectFile(which === 'psql' ? 'Select the psql executable' : 'Select the ffmpeg executable');
      if (!p) return;
      if (which === 'psql') c.psqlPath = p; else c.ffmpegPath = p;
      $cfg = c;
      await redetect();
    } catch {}
  }

  async function redetect() {
    if (!c) return;
    try { await SaveConfig(c); $env = await DetectEnvironment(); $savedTick++; } catch {}
  }

  function setTheme(t: string) {
    if (!c) return;
    c.theme = t;
    $cfg = c;
    applyTheme(t);
    UpdateTheme(t).catch(() => {});
  }

  async function test() {
    if (!conn) return;
    busy = true; testStatus = 'testing'; testMsg = '';
    try {
      await SaveConfig(c!); // persist so the backend tests the current values
      await TestConnection(conn.id);
      testStatus = 'ok'; testMsg = 'Connection OK.';
    } catch (e) { testStatus = 'error'; testMsg = String(e); }
    busy = false;
  }
</script>

{#if c}
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="wrap scroll" on:input={autoSave} on:change={autoSave}>
  <div class="content">
  <div class="grid">
    <div class="card">
      <h3>Database connections</h3>
      <div class="row" style="align-items:flex-end;">
        <div class="col">
          <label for="connsel">Active connection</label>
          <Select id="connsel" bind:value={c.selectedConnectionId} on:change={autoSave}
            options={c.connections.map((x) => ({ value: x.id, label: x.name }))} />
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
            <div style="width:110px;"><label for="cport">Port</label><Stepper id="cport" bind:value={conn.port} on:change={autoSave} /></div>
          </div>
          <div class="row" style="margin-top:10px;">
            <div class="col"><label for="cdb">Database</label><input id="cdb" bind:value={conn.dbName} /></div>
            <div class="col"><label for="cuser">User</label><input id="cuser" bind:value={conn.user} /></div>
          </div>
          <label for="cssl" style="margin-top:10px;">SSL mode</label>
          <Select id="cssl" options={sslModes} bind:value={conn.sslMode} on:change={autoSave} />

          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label style="margin-top:12px;">Session password
            <Hint text="Held in memory for this session only — never written to disk. Leave blank to use ~/.pgpass. Applied when you leave the field; empty it to clear." />
          </label>
          <div class="row" style="align-items:center;">
            <input type="password" bind:value={password} on:change={applyPassword}
              style="max-width:320px;" placeholder={pwStored ? '•••••••• (set)' : 'leave blank to use ~/.pgpass'} />
            <button class="ghost" on:click={test} disabled={busy}>Test connection</button>
            {#if testStatus === 'testing'}<span class="muted" style="font-size:0.8rem;">testing…</span>
            {:else if testStatus === 'ok'}<Hint label="✓" tone="ok" text={testMsg} />
            {:else if testStatus === 'error'}<Hint label="✗" tone="err" text={testMsg} />{/if}
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
          <Stepper id="dwell" min={0} bind:value={c.dwellSeconds} on:change={autoSave} />
        </div>
        <div class="col">
          <label for="prev">Preview rows on screen</label>
          <Stepper id="prev" min={0} bind:value={c.previewRows} on:change={autoSave} />
        </div>
      </div>

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

    <div class="card">
      <h3>Archive</h3>
      <label class="toggle"><input type="checkbox" bind:checked={c.zip} /> Create a ZIP archive after each run</label>

      <div style="margin-top:14px;" class:disabled-block={!c.zip}>
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label for="zipmode">Password protection
          <Hint text="Encryption uses legacy ZipCrypto — opens with macOS unzip, Windows Explorer and 7-Zip, but is cryptographically weak." />
        </label>
        <Select id="zipmode" options={zipModes} bind:value={c.zipPasswordMode} disabled={!c.zip} on:change={autoSave} />

        {#if c.zipPasswordMode === 'explicit'}
          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label for="zippw" style="margin-top:12px;">Explicit ZIP password
            <Hint text="Stored in plaintext in config.json. Leave blank to be prompted for a password after each run." />
          </label>
          <input id="zippw" type="password" bind:value={c.zipPassword} disabled={!c.zip}
            placeholder="leave blank to be prompted at run time" />
        {:else if c.zipPasswordMode === 'auto'}
          <div class="muted" style="font-size:0.78rem; margin-top:10px;">
            A random password is saved next to the archive as <span class="mono">&lt;name&gt;.zip.pwd</span>.
          </div>
        {/if}

        <label class="toggle" style="margin-top:14px;">
          <input type="checkbox" bind:checked={c.deleteSourcesAfterZip} disabled={!c.zip} />
          Delete source files after a successful ZIP (keep only the archive)
          <Hint text="After the archive is confirmed, the loose run files are removed, leaving only the .zip (and .pwd). Only runs when the archive exists." />
        </label>
        <label class="toggle" style="margin-top:8px;">
          <input type="checkbox" bind:checked={c.excludeVideoFromZip} disabled={!c.zip} />
          Exclude the video from the ZIP
          <Hint text="Large recordings can bloat the archive. When excluded, run.mp4 is left out of the ZIP and kept in the run folder even if 'Delete source files' is on." />
        </label>
      </div>
    </div>

    <div class="card">
      <h3>Appearance</h3>
      <!-- svelte-ignore a11y-label-has-associated-control -->
      <label>Theme</label>
      <div class="seg">
        {#each [['system', 'System'], ['light', 'Light'], ['dark', 'Dark']] as opt}
          <button class="segbtn" class:active={(c.theme || 'system') === opt[0]}
            on:click={() => setTheme(opt[0])}>{opt[1]}</button>
        {/each}
      </div>
    </div>

    <div class="card env">
      <h3>Environment</h3>
      <!-- svelte-ignore a11y-label-has-associated-control -->
      <label for="psqlpath">psql path
        <Hint text="Leave blank to auto-detect (PATH + common install dirs). Set a custom path if psql isn't found." />
      </label>
      <div class="row">
        <input id="psqlpath" bind:value={c.psqlPath} on:change={redetect}
          placeholder={$env?.psqlFound ? 'auto-detected' : 'auto-detect (not found)'} />
        <button class="ghost" on:click={() => browseTool('psql')}>Browse…</button>
      </div>
      <!-- svelte-ignore a11y-label-has-associated-control -->
      <label for="ffpath" style="margin-top:10px;">ffmpeg path
        <Hint text="Leave blank to auto-detect. Only needed for video recording." />
      </label>
      <div class="row">
        <input id="ffpath" bind:value={c.ffmpegPath} on:change={redetect}
          placeholder={$env?.ffmpegFound ? 'auto-detected' : 'auto-detect (not installed)'} />
        <button class="ghost" on:click={() => browseTool('ffmpeg')}>Browse…</button>
      </div>
      <table style="margin-top:14px;">
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
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; align-items: start; }
  .env { grid-column: 1 / -1; }
  h3 { margin: 0 0 12px; }
  .toggles { display: flex; flex-direction: column; gap: 8px; margin-top: 14px; }
  .toggle { display: flex; align-items: center; gap: 8px; margin: 0; color: var(--text); font-size: 0.9rem; }
  .toggle.disabled { opacity: 0.55; }
  .disabled-block { opacity: 0.5; }
  .seg { display: inline-flex; border: 1px solid var(--border-strong); border-radius: 8px; overflow: hidden; }
  .segbtn { border: none; border-radius: 0; background: transparent; padding: 7px 16px; }
  .segbtn:not(:disabled):hover { background: var(--btn-hover); }
  .segbtn + .segbtn { border-left: 1px solid var(--border-strong); }
  .segbtn.active { background: var(--accent-2); color: var(--on-accent); }
  .segbtn.active:not(:disabled):hover { background: var(--accent-hover); }
  table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
  td { padding: 4px 8px; border-bottom: 1px solid var(--border-strong); vertical-align: top; }
  td:first-child { color: var(--muted); width: 110px; }
</style>
