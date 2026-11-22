<script lang="ts">
  // Mooring's instructions for agents: the switch, the fixed text read-only, and extra text for the shown project.
  import { MAX_PROJECT_INSTRUCTIONS, type Settings } from './lib/settingsapi'
  import { settings } from './lib/settingsstore.svelte'
  import { app } from './lib/state.svelte'

  let { save }: { save: (change: Partial<Settings>) => Promise<void> } = $props()

  // A Go test (internal/chat/instructions_web_test.go) fails unless this string equals instructionsText in instructions.go.
  const SENT = 'You are running inside Mooring, a browser workspace. When output shortening is on for this chat, Mooring shortens long command output before you read it: it keeps the first and last lines and the lines that look like errors, and ends with a note such as "[mooring: 120 of 400 lines cut; ask the user for the full output]". When you see that note and need the part that was cut, run a narrower command (grep, head or tail, or write the output to a file and read part of it) instead of running the same command again.'

  const uid = $props.id()
  const project = $derived(app.current)
  const saved = $derived(project ? (settings.value.projectInstructions[project.id] ?? '') : '')
  let text = $state('')
  let shownFor = ''
  // The box follows the shown project and what the server holds, unless the user is typing in it.
  $effect(() => {
    const key = (project?.id ?? '') + '\n' + saved
    if (key !== shownFor) {
      shownFor = key
      text = saved
    }
  })

  const commit = () => {
    if (project && text !== saved) void save({ projectInstructions: { [project.id]: text } })
  }
  // Escape closes the dialog without a blur, so the box saves when it goes away too.
  const keep = () => () => commit()
</script>

<label class="row">
  <input type="checkbox" checked={settings.value.agentInstructions} onchange={(e) => save({ agentInstructions: e.currentTarget.checked })} />
  <span>
    <strong>Add Mooring's instructions to new chats</strong>
    <span class="help">Tells the agent how Mooring shortens long output and how to ask for the rest. Sent once when a chat starts. Never includes your files or secrets.</span>
  </span>
</label>

<details class="sent">
  <summary>What is sent</summary>
  <p class="help">{SENT}</p>
</details>

{#if project}
  <div class="field">
    <label for="{uid}-extra"><strong>Extra instructions for {project.name}</strong></label>
    <textarea id="{uid}-extra" class="input" rows="4" maxlength={MAX_PROJECT_INSTRUCTIONS} bind:value={text} onblur={commit} onchange={commit} aria-describedby="{uid}-count" {@attach keep()}></textarea>
    <span id="{uid}-count" class="help">{text.length} of {MAX_PROJECT_INSTRUCTIONS.toLocaleString('en-US')} characters. Added after the text above in new chats of this project. Saved when you leave the box.</span>
  </div>
{/if}

<style>
  .row {
    display: flex;
    gap: var(--s-3);
    align-items: flex-start;
    margin-bottom: var(--s-2);
  }
  .row input {
    margin-top: var(--s-1);
    flex: none;
  }
  .row span {
    display: grid;
    gap: var(--s-1);
  }
  .sent {
    margin: 0 0 var(--s-4) calc(var(--s-3) + 13px);
    font-size: var(--fs-s);
  }
  summary {
    min-height: var(--control-h);
    color: var(--muted);
  }
  .field {
    display: grid;
    gap: var(--s-1);
    margin-bottom: var(--s-4);
  }
  textarea {
    resize: vertical;
  }
</style>
