<script lang="ts">
  // A fully theme-controlled dropdown. We can't use a native <select> on Linux:
  // WebKitGTK draws its popup with the GTK theme (bright in our dark theme) and
  // ignores color-scheme. This renders our own button + menu, themed via CSS vars.
  // The menu is position:fixed (computed from the trigger rect) so it isn't clipped
  // by overflow containers — same trick as Hint.svelte.
  import { createEventDispatcher } from 'svelte';
  export let value: any;
  export let options: { value: any; label: string }[] = [];
  export let disabled = false;
  export let id: string | undefined = undefined;
  export let compact = false; // smaller height (e.g. the Run bar)

  const dispatch = createEventDispatcher();
  let open = false;
  let btn: HTMLButtonElement;
  let menuX = 0, menuY = 0, menuW = 0, maxH = 280, dropUp = false;

  $: selected = options.find((o) => o.value === value);

  function place() {
    const r = btn.getBoundingClientRect();
    menuX = r.left;
    menuW = r.width;
    const below = window.innerHeight - r.bottom - 8;
    const above = r.top - 8;
    dropUp = below < 160 && above > below;
    maxH = Math.max(120, Math.min(300, dropUp ? above : below));
    menuY = dropUp ? r.top : r.bottom;
  }
  function toggle() {
    if (disabled) return;
    if (open) { open = false; return; }
    place();
    open = true;
  }
  function pick(o: { value: any; label: string }) {
    value = o.value;
    open = false;
    dispatch('change');
  }
  function close() { open = false; }
  function onWinClick(e: MouseEvent) {
    if (open && btn && !btn.contains(e.target as Node)) open = false;
  }
</script>

<svelte:window
  on:click={onWinClick}
  on:keydown={(e) => e.key === 'Escape' && close()}
  on:resize={close}
  on:scroll|capture={close}
/>

<button
  {id}
  type="button"
  class="sel"
  class:open
  class:compact
  {disabled}
  bind:this={btn}
  on:click|stopPropagation={toggle}
>
  <span class="lbl">{selected ? selected.label : ''}</span>
  <span class="chev" aria-hidden="true">
    <svg viewBox="0 0 12 12" width="12" height="12"><path d="M2 4.5l4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
  </span>
</button>

{#if open}
  <div
    class="menu"
    style="left:{menuX}px; width:{menuW}px; max-height:{maxH}px; {dropUp ? `bottom:${window.innerHeight - menuY + 2}px` : `top:${menuY + 2}px`}"
  >
    {#each options as o}
      <button type="button" class="opt" class:active={o.value === value} on:click|stopPropagation={() => pick(o)}>
        {o.label}
      </button>
    {/each}
  </div>
{/if}

<style>
  .sel {
    width: 100%; height: 38px; display: flex; align-items: center; gap: 8px;
    background: var(--bg); border: 1px solid var(--border-strong); border-radius: 6px;
    padding: 0 10px; color: var(--text); font-size: 0.9rem; text-align: left;
  }
  .sel.compact { height: 30px; font-size: 0.85rem; padding: 0 8px; }
  .sel:not(:disabled):hover { background: var(--bg); border-color: var(--border-hover); }
  .sel.open { border-color: var(--accent); }
  .sel:disabled { opacity: 0.5; cursor: not-allowed; }
  .sel .lbl { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sel .chev { color: var(--muted); display: flex; flex: 0 0 auto; }
  .menu {
    position: fixed; z-index: 50; overflow: auto;
    background: var(--bg-2); border: 1px solid var(--border-strong); border-radius: 8px;
    box-shadow: 0 8px 24px #0007; padding: 4px;
  }
  .opt {
    width: 100%; text-align: left; background: transparent; border: none; border-radius: 5px;
    padding: 7px 10px; color: var(--text); font-size: 0.9rem; cursor: pointer; white-space: nowrap;
  }
  .opt:not(:disabled):hover { background: var(--btn-hover); }
  .opt.active { background: var(--accent-2); color: var(--on-accent); }
</style>
