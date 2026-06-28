<script lang="ts">
  // A number field with custom, theme-styled up/down chevrons. The native input
  // spin buttons can't be themed on WebKitGTK (they render grayish), so we hide
  // them (in style.css) and draw our own here.
  import { createEventDispatcher } from 'svelte';
  export let value: number;
  export let min: number | undefined = undefined;
  export let max: number | undefined = undefined;
  export let step = 1;
  export let id: string | undefined = undefined;
  export let disabled = false;

  const dispatch = createEventDispatcher();

  function clamp(v: number) {
    if (min !== undefined && v < min) v = min;
    if (max !== undefined && v > max) v = max;
    return v;
  }
  function bump(d: number) {
    if (disabled) return;
    value = clamp((Number(value) || 0) + d * step);
    dispatch('change');
  }
</script>

<div class="stepper" class:disabled>
  <input {id} type="number" bind:value {min} {max} {step} {disabled} on:change />
  <div class="btns">
    <button type="button" tabindex="-1" aria-label="Increase" {disabled} on:click={() => bump(1)}>
      <svg viewBox="0 0 12 8" width="9" height="6"><path d="M1 6l5-4 5 4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
    </button>
    <button type="button" tabindex="-1" aria-label="Decrease" {disabled} on:click={() => bump(-1)}>
      <svg viewBox="0 0 12 8" width="9" height="6"><path d="M1 2l5 4 5-4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
    </button>
  </div>
</div>

<style>
  .stepper { position: relative; width: 100%; }
  .stepper input { width: 100%; padding-right: 22px; }
  .btns {
    position: absolute; top: 1px; right: 6px; bottom: 1px; width: 14px;
    display: flex; flex-direction: column; justify-content: center; gap: 1px;
  }
  .btns button {
    flex: 0 0 auto; padding: 0; margin: 0; border: none; border-radius: 0; height: auto;
    background: transparent; color: var(--muted);
    display: flex; align-items: center; justify-content: center; cursor: pointer;
  }
  .btns button:not(:disabled):hover { background: transparent; color: var(--text); }
  .stepper.disabled { opacity: 0.5; }
  .stepper.disabled .btns button { cursor: not-allowed; }
</style>
