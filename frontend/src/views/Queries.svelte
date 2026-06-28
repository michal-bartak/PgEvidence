<script lang="ts">
  import { queries } from '../stores';
  import { store } from '../../wailsjs/go/models';
  import {
    SaveQuery, DeleteQuery, ReplaceAllQueries, ImportQueries, ExportQueries,
  } from '../../wailsjs/go/main/App';

  let selectedId: string | null = null;
  let draft: store.Query | null = null;
  let busy = false;
  let error = '';

  // import/export modal state
  let modal: 'import' | 'export' | null = null;
  let modalText = '';

  $: selected = $queries.find((q) => q.id === selectedId) ?? null;

  function newQuery() {
    selectedId = null;
    draft = store.Query.createFrom({ name: 'New query', sql: '', enabled: true });
  }

  function edit(q: store.Query) {
    selectedId = q.id;
    draft = store.Query.createFrom({ ...q });
  }

  async function save() {
    if (!draft) return;
    busy = true; error = '';
    try {
      $queries = await SaveQuery(draft);
      const match = $queries.find((q) => q.name === draft!.name && q.sql === draft!.sql);
      selectedId = match ? match.id : selectedId;
      draft = match ? store.Query.createFrom({ ...match }) : draft;
    } catch (e) { error = String(e); }
    busy = false;
  }

  async function toggle(q: store.Query) {
    try { $queries = await SaveQuery(store.Query.createFrom({ ...q, enabled: !q.enabled })); }
    catch (e) { error = String(e); }
  }

  async function remove(q: store.Query) {
    try {
      $queries = await DeleteQuery(q.id);
      if (selectedId === q.id) { selectedId = null; draft = null; }
    } catch (e) { error = String(e); }
  }

  // --- drag-and-drop reordering ---
  let dragIndex: number | null = null;
  let overIndex: number | null = null;

  function onDragStart(e: DragEvent, i: number) {
    dragIndex = i;
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move';
  }
  function onDragOver(e: DragEvent, i: number) {
    e.preventDefault();
    overIndex = i;
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
  }
  function onDragEnd() { dragIndex = null; overIndex = null; }

  async function onDrop(e: DragEvent, i: number) {
    e.preventDefault();
    const from = dragIndex;
    dragIndex = null; overIndex = null;
    if (from === null || from === i) return;
    const arr = [...$queries];
    const [moved] = arr.splice(from, 1);
    arr.splice(i, 0, moved);
    $queries = arr; // optimistic
    try { $queries = await ReplaceAllQueries(arr); } catch (e) { error = String(e); }
  }

  function openImport() { modal = 'import'; modalText = ''; error = ''; }

  async function openExport() {
    modal = 'export'; error = '';
    try { modalText = await ExportQueries(); } catch (e) { error = String(e); }
  }

  async function doImport() {
    busy = true; error = '';
    try {
      $queries = await ImportQueries(modalText);
      modal = null; selectedId = null; draft = null;
    } catch (e) { error = String(e); }
    busy = false;
  }
</script>

<div class="wrap">
  <div class="list card">
    <div class="toolbar">
      <button class="primary" on:click={newQuery}>+ Add query</button>
      <button class="ghost" on:click={openImport}>Import all…</button>
      <button class="ghost" on:click={openExport}>Export all…</button>
    </div>
    {#if error}<div class="error">{error}</div>{/if}
    {#if $queries.length === 0}
      <div class="muted empty">No queries yet. Add one or import a set.</div>
    {/if}
    <div class="items scroll">
      {#each $queries as q, i (q.id)}
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div
          class="item"
          class:sel={q.id === selectedId}
          class:off={!q.enabled}
          class:dragging={dragIndex === i}
          class:dropover={overIndex === i && dragIndex !== i}
          draggable="true"
          on:dragstart={(e) => onDragStart(e, i)}
          on:dragover={(e) => onDragOver(e, i)}
          on:drop={(e) => onDrop(e, i)}
          on:dragend={onDragEnd}
        >
          <span class="grip" title="Drag to reorder" aria-hidden="true">⠿</span>
          <input type="checkbox" checked={q.enabled} on:change={() => toggle(q)} title="Enabled" />
          <button class="name ghost" on:click={() => edit(q)}>
            <span class="idx">{i + 1}.</span> {q.name}
          </button>
          <div class="ops">
            <button class="danger mini" on:click={() => remove(q)} title="Delete">✕</button>
          </div>
        </div>
      {/each}
    </div>
  </div>

  <div class="editor card">
    {#if draft}
      <label for="qname">Name</label>
      <input id="qname" bind:value={draft.name} />
      <label for="qsql" style="margin-top:12px;">SQL</label>
      <textarea id="qsql" bind:value={draft.sql} rows="16" spellcheck="false"
        placeholder="SELECT ..."></textarea>
      <div class="row" style="margin-top:12px; align-items:center;">
        <label style="margin:0; display:flex; gap:6px; align-items:center;">
          <input type="checkbox" bind:checked={draft.enabled} /> Enabled
        </label>
        <div class="spacer"></div>
        <button class="primary" on:click={save} disabled={busy}>Save</button>
      </div>
    {:else}
      <div class="muted empty">Select a query to edit, or add a new one.</div>
    {/if}
  </div>
</div>

{#if modal}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="overlay" on:click|self={() => (modal = null)}>
    <div class="modal card">
      <h3>{modal === 'import' ? 'Import queries (replace all)' : 'Export queries'}</h3>
      <p class="muted">
        {#if modal === 'import'}
          Paste a JSON query set (from Export) or a plain SQL script. A SQL script is split
          on semicolons into individual queries. This replaces the entire current set.
        {:else}
          Copy this JSON to save your query set.
        {/if}
      </p>
      <textarea bind:value={modalText} rows="14" spellcheck="false"></textarea>
      {#if error}<div class="error">{error}</div>{/if}
      <div class="row" style="margin-top:12px;">
        <div class="spacer"></div>
        <button class="ghost" on:click={() => (modal = null)}>Close</button>
        {#if modal === 'import'}
          <button class="primary" on:click={doImport} disabled={busy}>Import & replace</button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .wrap { display: grid; grid-template-columns: 380px 1fr; gap: 16px; height: 100%; padding: 16px; }
  .list, .editor { display: flex; flex-direction: column; min-height: 0; }
  .toolbar { display: flex; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
  .items { display: flex; flex-direction: column; gap: 4px; }
  .item {
    display: flex; align-items: center; gap: 8px;
    padding: 6px 8px; border-radius: 6px; border: 1px solid transparent;
  }
  .item:hover { background: var(--bg-2); }
  .item.sel { background: var(--bg-2); border-color: var(--border-strong); }
  .item.off { opacity: 0.5; }
  .item.dragging { opacity: 0.4; }
  .item.dropover { border-color: var(--accent); box-shadow: inset 0 2px 0 var(--accent); }
  .grip { cursor: grab; color: var(--muted); user-select: none; padding: 0 2px; font-size: 1rem; line-height: 1; }
  .grip:active { cursor: grabbing; }
  .name { flex: 1; text-align: left; background: transparent; border: none; padding: 4px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .idx { color: var(--muted); margin-right: 4px; }
  .ops { display: flex; gap: 3px; }
  .mini { padding: 2px 7px; font-size: 0.8rem; }
  .empty { padding: 24px; text-align: center; }
  .error { color: var(--err); font-size: 0.85rem; margin: 6px 0; }
  textarea { flex: 1; }
  .overlay {
    position: fixed; inset: 0; background: #0008; display: flex;
    align-items: center; justify-content: center; z-index: 10;
  }
  .modal { width: 640px; max-width: 90vw; }
  .modal h3 { margin: 0 0 6px; }
  .modal textarea { width: 100%; }
</style>
