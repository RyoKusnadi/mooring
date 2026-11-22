<script lang="ts">
  // What one chat's agent changed in its last turn: the files, and the diff of the one picked. Read-only.
  import TurnDiff from './TurnDiff.svelte'
  import CommentBar from './CommentBar.svelte'
  import { chats } from './lib/chatstore.svelte'
  import { gitError } from './lib/gitapi'
  import { baseName } from './lib/paths'
  import { settings } from './lib/settingsstore.svelte'
  import { app } from './lib/state.svelte'
  import { getLastTurn, kindWord, noTurnText, type LastTurn, type TurnFile } from './lib/turnapi'

  let { chat }: { chat: string } = $props()

  const info = $derived(chats.list[chat])
  const name = $derived(info?.title || 'This chat')
  const off = $derived(!!settings.value.checkpointsOff[app.currentId])
  let data = $state<LastTurn | null>(null)
  let error = $state('')
  let picked = $state('')
  const files = $derived(data?.files ?? [])
  const shown = $derived<TurnFile | undefined>(files.find((f) => f.path === picked) ?? files[0])

  // Read again when this chat changes: a turn ends, or a new one starts.
  let seq = 0
  let shownChat = ''
  $effect(() => {
    void info?.updatedAt
    void info?.state
    const mine = ++seq
    const project = app.currentId
    if (chat !== shownChat) {
      shownChat = chat
      data = null
      error = ''
    }
    if (!chat || !project) return void (data = null)
    getLastTurn((n, a) => app.run(n, a), project, chat).then(
      (r) => {
        if (mine !== seq) return
        data = { turn: r.turn, files: r.files ?? [] }
        error = ''
      },
      (e: unknown) => {
        if (mine === seq) error = gitError(e, 'Could not read the last turn. Try again.').message
      },
    )
  })
  const dir = (p: string) => (p.includes('/') ? p.slice(0, p.lastIndexOf('/')) : '')
</script>

<div class="turn" data-last-turn>
  {#if error}
    <p class="note error" role="alert">{error}</p>
  {:else if !data}
    <p class="note" role="status">Reading the last turn…</p>
  {:else if !data.turn}
    <p class="note" role="status">{noTurnText(name, off)}</p>
  {:else}
    <div class="grid">
      <div class="list">
        <p class="sub" role="status">
          {files.length === 0 ? `${name} changed no files in its last turn.` : `${files.length} ${files.length === 1 ? 'file' : 'files'} changed by ${name} in its last turn.`}
        </p>
        <ul aria-label="Files changed in the last turn">
          {#each files as f (f.path)}
            <li>
              <button type="button" class="file" aria-current={shown?.path === f.path} onclick={() => (picked = f.path)} title={f.path}>
                <span class="nm">{baseName(f.path)}{#if dir(f.path)}<span class="dir"> {dir(f.path)}</span>{/if}</span>
                <span class="kind">{kindWord(f.kind)}</span>
                <span class="cnt"><span class="plus">+{f.added}</span> <span class="minus">−{f.removed}</span></span>
              </button>
            </li>
          {/each}
        </ul>
      </div>
      <div class="diff">
        {#if shown}
          {#key chat + ':' + data.turn.end + ':' + shown.path}<TurnDiff {chat} file={shown} />{/key}
        {/if}
      </div>
    </div>
  {/if}
  <CommentBar />
</div>

<style>
  .turn {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
  }
  .grid {
    display: grid;
    flex: 1;
    grid-template-columns: clamp(280px, 32%, 480px) minmax(0, 1fr);
    min-height: 0;
  }
  .list {
    overflow-y: auto;
    border-right: 1px solid var(--border);
  }
  .diff {
    min-width: 0;
    min-height: 0;
  }
  .note,
  .sub {
    margin: 0;
    padding: var(--s-3) var(--s-4);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .note.error {
    color: var(--danger);
  }
  ul {
    margin: 0;
    padding: 0 var(--s-2) var(--s-2);
    list-style: none;
  }
  .file {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    width: 100%;
    min-height: var(--control-h);
    padding: 0 var(--s-2);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .file:hover {
    background: var(--hover);
  }
  .file[aria-current='true'] {
    background: var(--selected);
  }
  .nm {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dir,
  .kind {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .plus {
    color: var(--success);
  }
  .minus {
    color: var(--danger);
  }
  .cnt {
    flex: none;
    font-size: var(--fs-s);
  }
  @media (max-width: 900px) {
    .grid {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: minmax(80px, 35%) minmax(0, 1fr);
    }
    .list {
      border-right: 0;
      border-bottom: 1px solid var(--border);
    }
  }
</style>
