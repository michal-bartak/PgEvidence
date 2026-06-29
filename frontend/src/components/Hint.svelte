<script lang="ts">
  // A small "?" that reveals help text on hover/focus. The popover is fixed-
  // positioned (computed from the button rect) so it is never clipped by
  // overflow:hidden containers, and clamped to the viewport so it can't stick
  // out past a window edge (e.g. hints near the right border).
  import { tick } from 'svelte';
  export let text = '';
  export let label = '?';        // glyph shown in the badge
  export let tone: '' | 'ok' | 'err' = '';
  let open = false;
  let left = 0;
  let top = 0;
  let positioned = false;
  let btn: HTMLButtonElement;
  let pop: HTMLDivElement;

  async function show() {
    open = true;
    positioned = false;
    await tick(); // wait for the popover to render so we can measure it
    if (!btn || !pop) return;
    const m = 8; // viewport margin
    const r = btn.getBoundingClientRect();
    const pw = pop.offsetWidth;
    const ph = pop.offsetHeight;
    // Centre on the badge, then clamp horizontally inside the viewport.
    left = Math.max(m, Math.min(r.left + r.width / 2 - pw / 2, window.innerWidth - pw - m));
    // Prefer below; flip above if it would overflow the bottom.
    top = r.bottom + 6;
    if (top + ph > window.innerHeight - m) top = r.top - ph - 6;
    if (top < m) top = m;
    positioned = true;
  }
  function hide() {
    open = false;
    positioned = false;
  }
</script>

<button
  bind:this={btn}
  class="q {tone}"
  type="button"
  aria-label={text || 'More information'}
  on:mouseenter={show}
  on:mouseleave={hide}
  on:focus={show}
  on:blur={hide}
  on:click|preventDefault
><span class="glyph">{label}</span></button>

{#if open}
  <div class="pop" bind:this={pop} style="left:{left}px; top:{top}px; opacity:{positioned ? 1 : 0};">{text}<slot /></div>
{/if}

<style>
  .q {
    width: 16px; height: 16px; padding: 0; line-height: 1; flex: 0 0 auto;
    border-radius: 50%; font-size: 0.72rem; cursor: help;
    color: var(--muted); background: var(--bg-3);
    border: 1px solid var(--border-strong);
    display: inline-flex; align-items: center; justify-content: center;
    /* middle aligns to x-height, which reads ~1px low next to cap-height text;
       relative nudge raises the badge to the optical centre. */
    vertical-align: middle; position: relative; top: -1px;
  }
  /* The "?" glyph sits optically high in the em-box; nudge it down ~1px so it
     reads as vertically centred within the badge. */
  .glyph { position: relative; top: 1px; line-height: 1; }
  .q:hover { color: var(--text); }
  .q.ok { color: var(--ok); border-color: var(--ok); }
  .q.err { color: var(--err); border-color: var(--err); }
  .q.ok:hover { color: var(--ok); }
  .q.err:hover { color: var(--err); }
  .pop {
    position: fixed;
    max-width: 280px;
    background: var(--bg-3); color: var(--text);
    border: 1px solid var(--border-strong); border-radius: 8px;
    padding: 8px 11px; font-size: 0.8rem; line-height: 1.45;
    z-index: 100; box-shadow: 0 6px 20px #0006; pointer-events: none;
    white-space: normal;
  }
</style>
