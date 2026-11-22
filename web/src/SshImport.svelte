<script lang="ts">
  // Pick hosts from ~/.ssh/config to save in Mooring. The file is read, never changed.
  import { onMount } from 'svelte'
  import { fetchConfig, type ConfigHost } from './lib/sshapi'
  import { draftOfConfig, hostLabel } from './lib/sshfmt'
  import { ssh } from './lib/sshstore.svelte'

  let { onDone, onCancel }: { onDone: () => void; onCancel: () => void } = $props()

  let entries = $state<ConfigHost[] | null>(null)
  let picked = $state<Record<string, boolean>>({})
  let error = $state('')
  let busy = $state(false)
  let results = $state<string[]>([])

  const known = (c: ConfigHost) => ssh.hosts.some((h) => h.address === c.address && h.port === (c.port || 22) && h.user === c.user)
  const count = $derived(Object.values(picked).filter(Boolean).length)

  onMount(() => {
    fetchConfig().then(
      (r) => (entries = r.hosts ?? []),
      () => (error = 'Could not read your SSH config. Check that Mooring is running.'),
    )
  })

  async function save() {
    busy = true
    error = ''
    results = []
    for (const c of entries ?? []) {
      if (!picked[c.name]) continue
      try {
        await ssh.saveHost(draftOfConfig(c))
        picked[c.name] = false
      } catch (e) {
        results.push(`${c.name}: ${e instanceof Error ? e.message : 'not saved'}`)
      }
    }
    busy = false
    if (results.length === 0) onDone()
  }
</script>

<p class="help">Hosts from <code>~/.ssh/config</code>. Pick the ones to save. The file stays as it is.</p>
{#if error}<p class="error" role="alert">{error}</p>{/if}
{#if entries === null && !error}
  <p class="muted" role="status">Reading your SSH config…</p>
{:else if entries && entries.length === 0}
  <p class="muted">Your SSH config has no hosts to import.</p>
{:else if entries}
  <ul>
    {#each entries as c (c.name)}
      <li>
        <label class="pick">
          <input type="checkbox" bind:checked={picked[c.name]} disabled={!!c.skipped} />
          <span class="what">
            <strong>{c.name}</strong>
            <span class="mono">{hostLabel({ ...c, port: c.port || 22 })}</span>
            {#if c.skipped}<span class="skip">Not imported: {c.skipped}</span>{:else if known(c)}<span class="muted">Already saved. Importing adds a second entry.</span>{/if}
          </span>
        </label>
      </li>
    {/each}
  </ul>
{/if}
{#each results as r (r)}<p class="error" role="alert">{r}</p>{/each}
<div class="acts">
  <button type="button" class="btn" data-first onclick={onCancel}>Back</button>
  <button type="button" class="btn primary" disabled={busy || count === 0} onclick={save}>Import {count > 0 ? count : ''} {count === 1 ? 'host' : 'hosts'}</button>
</div>

<style>
  ul {
    margin: var(--s-3) 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border);
  }
  li {
    border-bottom: 1px solid var(--border);
  }
  .pick {
    display: flex;
    align-items: flex-start;
    gap: var(--s-3);
    min-height: var(--control-h);
    padding: var(--s-2) 0;
  }
  .pick input {
    flex: none;
    width: 18px;
    height: 18px;
    margin: 2px 0 0;
  }
  .what {
    display: flex;
    flex-direction: column;
    min-width: 0;
    gap: var(--s-1);
    font-size: var(--fs-s);
  }
  .what strong {
    font-size: var(--fs-m);
  }
  .mono {
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }
  .muted {
    color: var(--muted);
  }
  .skip {
    color: var(--attention);
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--s-2);
  }
  p {
    margin: 0 0 var(--s-2);
  }
</style>
