<script lang="ts">
  // Token-saving section of Settings. Each change is saved as it is made; the server answers with the whole
  // settings and tells other devices.
  import SettingsCheckpoints from './SettingsCheckpoints.svelte'
  import SettingsInstructions from './SettingsInstructions.svelte'
  import SettingsModels from './SettingsModels.svelte'
  import { MAX_JOB_TOKENS, MIN_JOB_TOKENS, type Settings } from './lib/settingsapi'
  import { settings } from './lib/settingsstore.svelte'

  let error = $state('')
  let saved = $state(false)
  let limit = $state(String(settings.value.jobTokensPerDay))
  const uid = $props.id()

  async function save(change: Partial<Settings>) {
    error = ''
    saved = false
    try {
      await settings.save(change)
      saved = true
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not save the settings.'
    }
  }

  function saveLimit() {
    const n = Number(limit)
    if (!Number.isInteger(n) || n < MIN_JOB_TOKENS || n > MAX_JOB_TOKENS) {
      error = `The daily limit is a whole number from ${MIN_JOB_TOKENS.toLocaleString('en-US')} to ${MAX_JOB_TOKENS.toLocaleString('en-US')}.`
      return
    }
    if (n !== settings.value.jobTokensPerDay) void save({ jobTokensPerDay: n })
    else error = ''
  }
</script>

{#if error}<p class="error" role="alert">{error}</p>{/if}

<label class="row">
  <input type="checkbox" checked={settings.value.compactDefault} onchange={(e) => save({ compactDefault: e.currentTarget.checked })} />
  <span>
    <strong>Shorten long command output in new chats</strong>
    <span class="help">The agent gets the start and the end of output that is hundreds of lines long, and a note of how much was cut. The full output stays here, one click away. You can change this per chat.</span>
  </span>
</label>

<label class="row">
  <input type="checkbox" checked={settings.value.titleJobs} onchange={(e) => save({ titleJobs: e.currentTarget.checked })} />
  <span>
    <strong>Name chats with a small model</strong>
    <span class="help">After the first reply, a small model writes a short title for a chat the agent did not name. It uses tokens from the daily limit.</span>
  </span>
</label>

<label class="row">
  <input type="checkbox" checked={settings.value.commitJobs} onchange={(e) => save({ commitJobs: e.currentTarget.checked })} />
  <span>
    <strong>Suggest commit messages</strong>
    <span class="help">Adds a Suggest message button to the commit box. A small model reads the staged changes and writes a message. It uses tokens from the daily limit.</span>
  </span>
</label>

<SettingsInstructions {save} />
<SettingsCheckpoints {save} />

<div class="field">
  <label for="{uid}-limit"><strong>Daily token limit for small jobs</strong></label>
  <input id="{uid}-limit" class="input num" inputmode="numeric" bind:value={limit} onchange={saveLimit} aria-describedby="{uid}-limit-help" />
  <span id="{uid}-limit-help" class="help">Titles and commit messages stop for the day when the total passes this. Chats are not counted.</span>
</div>

<h3>Small model per agent</h3>
<p class="help lead">Titles and commit messages run on the model you pick here. An agent with no model set does not do these jobs.</p>
<SettingsModels {save} />

<p class="status" role="status">{saved && !error ? 'Saved.' : ''}</p>

<style>
  .row {
    display: flex;
    gap: var(--s-3);
    align-items: flex-start;
    margin-bottom: var(--s-4);
  }
  .row input {
    margin-top: var(--s-1);
    flex: none;
  }
  .row span {
    display: grid;
    gap: var(--s-1);
  }
  .field {
    display: grid;
    gap: var(--s-1);
    margin-bottom: var(--s-4);
  }
  .num {
    width: 12ch;
  }
  h3 {
    margin: var(--s-4) 0 var(--s-1);
    font-size: var(--fs-m);
  }
  .lead {
    margin: 0 0 var(--s-3);
  }
  .status {
    min-height: 1.2em;
    margin: var(--s-3) 0 0;
    color: var(--success);
  }
</style>
