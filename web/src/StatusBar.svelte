<script lang="ts">
  import { actions } from './lib/actions'
  import { app } from './lib/state.svelte'
  import { chats } from './lib/chatstore.svelte'
  import { terms } from './lib/termstore.svelte'
  import { needText } from './lib/attention'
  import { waitingText } from './lib/chatfmt'
  import { lsp } from './lib/lspstore.svelte'
  import { lspStatus } from './lib/lspstatus'

  const chatsWaiting = $derived(chats.attentionCount())
  const termsWaiting = $derived(terms.attentionCount())
  const progress = $derived(lspStatus(lsp.servers, lsp.progress, lsp.conn))
  const first = $derived(Object.keys(chats.list).find((id) => chats.list[id].attention))
  const firstTerm = $derived(Object.keys(terms.list).find((id) => terms.needs(id)))
</script>

<footer>
  {#if app.status === 'live'}
    <span class="quiet" role="status">Connected</span>
  {:else}
    <span class="loud" role="status">
      {app.status === 'connecting' ? 'Connecting to the server…' : 'Reconnecting to the server… Changes are kept and sent when it is back.'}
    </span>
  {/if}
  {#if progress}<span class="info" role="status" title={progress}>{progress}</span>{/if}
  {#if app.info}<span class="info" role="status" title={app.info}>{app.info}</span>{/if}
  {#if app.notice}<span class="error" role="alert">{app.notice}</span>{/if}
  {#if chatsWaiting + termsWaiting > 0}
    <button type="button" class="attn" onclick={() => (first ? actions.openChat(first) : firstTerm && actions.openTerminalTab(firstTerm))}><span class="dot" aria-hidden="true"></span>{needText(chatsWaiting, termsWaiting, waitingText(Object.values(chats.list)))}</button>
  {/if}
</footer>

<style>
  footer {
    display: flex;
    align-items: center;
    gap: var(--s-4);
    min-height: 28px;
    padding: 0 max(var(--s-3), env(safe-area-inset-right, 0px)) env(safe-area-inset-bottom, 0px) max(var(--s-3), env(safe-area-inset-left, 0px));
    overflow: hidden;
    background: var(--surface);
    border-top: 1px solid var(--border);
    font-size: var(--fs-s);
    white-space: nowrap;
  }
  .quiet {
    color: var(--muted);
  }
  .attn {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    margin-left: auto;
    padding: 0;
    border: 0;
    background: none;
    font: inherit;
    text-decoration: underline;
    color: var(--attention);
    font-weight: 600;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--attention);
  }
  /* The footer clips, so the focus ring is drawn inside the button. */
  .attn:focus-visible {
    outline-offset: -2px;
  }
  @media (pointer: coarse) {
    .attn {
      min-height: 44px;
      padding: 0 var(--s-2);
    }
  }
  /* A long notice, such as a language server restart, wraps instead of running off a phone screen. */
  .error {
    min-width: 0;
    padding: var(--s-1) 0;
    overflow-wrap: anywhere;
    white-space: normal;
  }
  /* One line like the rest of the bar, cut with an ellipsis; the full text is in the title. */
  .info {
    min-width: 0;
    overflow: hidden;
    color: var(--muted);
    text-overflow: ellipsis;
  }
  .loud {
    color: var(--fg);
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
