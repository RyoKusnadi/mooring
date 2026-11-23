<script lang="ts">
  // One line per tool call. An edit shows its diff under the line, a command its last 10 lines.
  // The line opens the rest: where it ran, what it was given, everything it printed.
  import { blocks, TOOL_STATUS } from './lib/chatfmt'
  import { editsOf, lastLines, outputText, stateLabel, stripAnsi, toolLine } from './lib/chattool'
  import { chatPlace, chatRoot } from './lib/chatplace'
  import { actions } from './lib/actions'
  import { chats } from './lib/chatstore.svelte'
  import { app } from './lib/state.svelte'
  import ChatOutputs from './ChatOutputs.svelte'
  import Icon from './Icon.svelte'
  import type { ChatMessage } from './lib/chatwire'

  let { msg, chatId }: { msg: ChatMessage; chatId: string } = $props()

  let open = $state(false)
  const info = $derived(chats.list[chatId])
  const projectPath = $derived(app.projects.find((p) => p.id === info?.projectId)?.path ?? '')
  const root = $derived(chatRoot(info, projectPath))
  const status = $derived(msg.status ?? 'pending')
  const line = $derived(toolLine(msg, root))
  const edits = $derived(editsOf(msg, root))
  const diffs = $derived(blocks(msg.content).filter((b) => b.kind === 'diff'))
  const printed = $derived(msg.toolKind === 'execute' ? stripAnsi(outputText(msg)) : '')
  const tail = $derived(printed ? lastLines(printed, 10) : null)
  const input = $derived(open && msg.rawInput !== undefined ? JSON.stringify(msg.rawInput, null, 2) : '')
  const rest = $derived(open ? blocks(msg.content).filter((b) => b.kind === 'text') : [])

  const openDiff = (path: string) => actions.openDiff(info ? chatPlace(chatId, info, projectPath) : '', path, false)
</script>

<div class="tool" class:failed={status === 'failed'}>
  <button type="button" class="line" aria-expanded={open} aria-label={`${line}. ${TOOL_STATUS[status] ?? status}. Details`} onclick={() => (open = !open)}>
    <Icon name={open ? 'chevron' : 'chevronRight'} />
    <span class="text">{line}</span>
    {#if (status === 'pending' || status === 'in_progress') && stateLabel(line, TOOL_STATUS[status])}<span class="state">{TOOL_STATUS[status]}</span>{/if}
  </button>
  {#if edits.length}
    <div class="files">
      {#each edits as e (e.path)}
        <button type="button" class="btn small ghost" onclick={() => openDiff(e.path)}>Open diff<span class="sr"> of {e.path}</span></button>
      {/each}
    </div>
  {/if}
  {#each diffs as b, i (i)}
    <pre class="diff" aria-label={`Changes to ${b.path}`}>{#each b.lines ?? [] as l, j (j)}<span class={l.sign === '+' ? 'add' : l.sign === '-' ? 'del' : ''}>{l.sign} {l.text}</span>{/each}</pre>
  {/each}
  {#if tail}
    <pre class="tail" aria-label="Last lines of output">{#if tail.more}<span class="more">…
</span>{/if}{tail.lines.join('\n')}</pre>
  {/if}
  {#if msg.outputs?.length}<ChatOutputs {chatId} outputs={msg.outputs} />{/if}
  {#if open}
    <div class="detail">
      {#if msg.locations?.length}
        <p class="meta">{msg.locations.map((l) => (l.line ? `${l.path}:${l.line}` : l.path)).join(', ')}</p>
      {/if}
      {#if input}<pre>{input}</pre>{/if}
      {#if tail?.more}<pre>{printed}</pre>{/if}
      {#each rest as b, i (i)}
        {#if msg.toolKind !== 'execute'}<pre>{b.text}</pre>{/if}
      {/each}
      {#if msg.truncated}<p class="meta">The output was too long and was cut.</p>{/if}
      {#if !input && !diffs.length && !tail && rest.length === 0 && !msg.locations?.length}<p class="meta">No details were sent for this call.</p>{/if}
    </div>
  {/if}
</div>

<style>
  .tool {
    display: grid;
    gap: var(--s-1);
    min-width: 0;
  }
  .line {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    min-height: 28px;
    padding: 0 var(--s-1);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--muted);
    font: inherit;
    text-align: left;
  }
  @media (pointer: coarse) {
    .line {
      min-height: 44px;
    }
  }
  .line:hover {
    background: var(--hover);
    color: var(--fg);
  }
  .text {
    min-width: 0;
    overflow: hidden;
    font-family: var(--font-mono);
    font-size: var(--fs-s);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .failed .text {
    color: var(--danger);
  }
  .state {
    flex: none;
    font-size: var(--fs-s);
  }
  .files {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1);
    padding-left: var(--s-1);
  }
  .detail {
    display: grid;
    gap: var(--s-2);
  }
  pre {
    max-height: 240px;
    margin: 0;
    padding: var(--s-2) var(--s-3);
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--r-m);
    background: var(--surface);
    font: var(--fs-code) / var(--lh-code) var(--font-mono);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .tail {
    max-height: calc(var(--lh-code) * 11 + var(--s-4));
  }
  .more {
    color: var(--muted);
  }
  .meta {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .diff span {
    display: block;
    min-height: var(--lh-code);
  }
  .add {
    background: var(--diff-add);
  }
  .del {
    background: var(--diff-del);
  }
</style>
