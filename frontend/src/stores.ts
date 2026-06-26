import { writable } from 'svelte/store';
import { config, store, main } from '../wailsjs/go/models';

export type Tab = 'queries' | 'run' | 'settings';

export const activeTab = writable<Tab>('queries');
export const cfg = writable<config.Config | null>(null);
export const queries = writable<store.Query[]>([]);
export const env = writable<main.EnvInfo | null>(null);

/** Per-run options shown/edited on the Run page. Initialized from saved config at
 *  app start; changes here apply to the run only and are never persisted. */
export interface RunOpts {
  screenshots: boolean;
  video: boolean;
  zip: boolean;
  deleteSourcesAfterZip: boolean;
  excludeVideoFromZip: boolean;
  connectionId: string;
}
export const runOpts = writable<RunOpts | null>(null);

/** Bumped after each Settings auto-save; App.svelte flashes the Settings tab. */
export const savedTick = writable(0);

/** Shape of the run:result event payload emitted by the Go runner. */
export interface ResultPayload {
  index: number;
  total: number;
  name: string;
  sql: string;
  sha256: string;
  header: string[] | null;
  rows: string[][] | null;
  rowCount: number;
  status: string;
  error: string;
  durationMs: number;
  resultFile: string;
}

export interface QueryPayload {
  index: number;
  total: number;
  name: string;
  sql: string;
}

export interface DonePayload {
  runDir?: string;
  manifestFile?: string;
  ok?: number;
  failed?: number;
  cancelled?: boolean;
  records?: any[];
  error?: string;
}
