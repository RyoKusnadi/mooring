<script lang="ts">
  // Search the catalog of tool servers (the official registry). Shows facts only: what the entry says,
  // and the ways Mooring can run it. Nothing is added until the consent screen.
  import { onMount } from 'svelte'
  import { tools } from './lib/toolstore.svelte'
  import { fetchCatalog, type CatalogEntry, type ServerDraft } from './lib/toolapi'
  import { dateOf, draftOfEntry, kindLabel, kindLine, needs, safeUrl } from './lib/toolfmt'

  let { onBack, onPick }: { onBack: () => void; onPick: (d: ServerDraft, missing: string) => void } = $props()

  let q = $state('')
  let entries = $state<CatalogEntry[]>([])
  let next = $state<number | null>(null)
  let date = $state('')
  let loading = $state(false)
  let error = $state('')
  let updating = $state(false)
  let note = $state('')
  let timer: ReturnType<typeof setTimeout> | undefined
  let seq = 0
  let ctl: AbortController | undefined

  async function load(more: boolean) {
    const mine = ++seq
    ctl?.abort()
    ctl = new AbortController()
    loading = true
    error = ''
    try {
      const p = await fetchCatalog(q.trim(), more ? next : null, undefined, ctl.signal)
      if (mine !== seq) return
      entries = more ? [...entries, ...p.entries] : p.entries
      next = p.next
      date = p.date
      loading = false
    } catch (e) {
      if (mine !== seq || (e instanceof DOMException && e.name === 'AbortError')) return
      error = e instanceof Error ? e.message : 'Could not read the catalog. Try again.'
      loading = false
    }
  }

  onMount(() => {
    void load(false)
    return () => {
      clearTimeout(timer)
      ctl?.abort()
    }
  })

  function typed() {
    clearTimeout(timer)
    timer = setTimeout(() => void load(false), 250)
  }

  async function update() {
    updating = true
    note = ''
    error = ''
    try {
      const r = await tools.cmd<{ count: number; date: string }>('toolServer.catalogUpdate', {})
      note = `Catalog updated: ${r.count} servers, as of ${dateOf(r.date)}.`
      await load(false)
    } catch (e) {
      error = (e instanceof Error ? e.message : 'Could not fetch the newer catalog.') + ' The catalog you have stays in use.'
    } finally {
      updating = false
    }
  }
</script>

<div class="top">
  <button type="button" class="btn" onclick={onBack}>Back to list</button>
</div>
<form onsubmit={(e) => { e.preventDefault(); clearTimeout(timer); void load(false) }} role="search">
  <div class="field">
    <label for="tc-q">Search by name or description</label>
    <input id="tc-q" class="input" type="search" bind:value={q} oninput={typed} autocomplete="off" spellcheck="false" data-first />
  </div>
</form>
<div class="meta">
  <span class="help" role="status">{loading && entries.length === 0 ? 'Loading…' : date ? `Catalog from the official registry, as of ${dateOf(date)}.` : ''}</span>
  <button type="button" class="btn" disabled={updating} onclick={update}>{updating ? 'Fetching…' : 'Check for a newer catalog'}</button>
</div>
{#if note}<p class="help" role="status">{note}</p>{/if}
{#if error}<p class="error" role="alert">{error}</p>{/if}
{#if !loading && !error && entries.length === 0}<p class="help">Nothing in the catalog matches that. Try fewer words, or add your own server.</p>{/if}
<ul>
  {#each entries as e (e.name)}
    {@const site = safeUrl(e.website)}
    {@const repo = safeUrl(e.repository)}
    <li>
      <div class="head">
        <strong>{e.title || e.name}</strong>
        <span class="muted">{e.name}, version {e.version}</span>
      </div>
      {#if e.description}<p class="desc">{e.description}</p>{/if}
      {#if site || repo}
        <p class="links">
          {#if site}<a href={site} target="_blank" rel="noopener noreferrer">Website<span class="sr"> of {e.title || e.name}</span></a>{/if}
          {#if repo}<a href={repo} target="_blank" rel="noopener noreferrer">Repository<span class="sr"> of {e.title || e.name}</span></a>{/if}
        </p>
      {/if}
      {#each e.kinds as k, i (i)}
        <div class="way">
          <div class="how">
            <span class="kind">{kindLabel(k)}{#if !k.available}{' '}<span class="miss">({needs(k)})</span>{/if}</span>
            <code>{kindLine(k)}</code>
          </div>
          <button type="button" class="btn" aria-label="Add {e.title || e.name}, run with {kindLabel(k)}" onclick={() => onPick(draftOfEntry(e, k), k.available ? '' : (k.missing ?? 'The program'))}>Add</button>
        </div>
      {/each}
    </li>
  {/each}
</ul>
{#if next !== null}<button type="button" class="btn" aria-busy={loading} onclick={() => !loading && load(true)}>{loading ? 'Loading…' : 'Show more'}</button>{/if}

<style>
  .top,
  .meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
    margin-bottom: var(--s-3);
  }
  form {
    margin-bottom: var(--s-2);
  }
  p {
    margin: 0 0 var(--s-2);
  }
  ul {
    margin: 0 0 var(--s-3);
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border);
  }
  li {
    display: grid;
    gap: var(--s-2);
    padding: var(--s-3) 0;
    border-bottom: 1px solid var(--border);
  }
  .head {
    display: grid;
    gap: var(--s-1);
    min-width: 0;
  }
  strong,
  .muted,
  .desc {
    overflow-wrap: anywhere;
  }
  .muted {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .links {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1) var(--s-4);
    font-size: var(--fs-s);
  }
  .links a {
    display: inline-flex;
    align-items: center;
    min-height: var(--control-h);
    color: var(--accent);
  }
  .way {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-3);
    padding: var(--s-2) var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-s);
    background: var(--surface);
  }
  .how {
    display: grid;
    gap: var(--s-1);
    min-width: 0;
    font-size: var(--fs-s);
  }
  .kind {
    font-weight: 600;
  }
  .miss {
    color: var(--attention);
  }
  code {
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }
  .btn {
    flex: none;
  }
</style>
