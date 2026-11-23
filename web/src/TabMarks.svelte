<script lang="ts">
  // Marks at the left of a tab: a chat or terminal that needs attention, a file with unsaved edits, a tab on another
  // computer (SSH) and a pinned tab.
  import Icon from './Icon.svelte'
  import { chats } from './lib/chatstore.svelte'
  import { membersOf } from './lib/chatgroups'
  import { files } from './lib/filestore.svelte'
  import { ssh } from './lib/sshstore.svelte'
  import { terms } from './lib/termstore.svelte'

  let { id, host = '', pinned = false }: { id: string; host?: string; pinned?: boolean } = $props()
  const hostName = $derived(host ? (ssh.host(host)?.name ?? 'a remote host') : '')
</script>

{#if chats.list[id]?.attention || membersOf(chats.list, id).some((m) => m.attention)}<span class="dot" role="img" aria-label="Needs permission" title="Needs permission"></span>{/if}{#if terms.needs(id)}<span class="dot" role="img" aria-label="Needs you" title="Needs you"></span>{/if}{#if files.dirty[id]}<span class="mod" role="img" aria-label="Unsaved changes" title="Unsaved changes"></span>{/if}{#if host}<span class="chip" role="img" aria-label="SSH" title="On {hostName} (SSH)">SSH</span>{/if}{#if pinned}<span class="mark" role="img" aria-label="Pinned" title="Pinned"><Icon name="pin" /></span>{/if}

<style>
  .dot,
  .mod {
    display: inline-block;
    flex: none;
    width: 8px;
    height: 8px;
    margin-right: var(--s-2);
    border-radius: 50%;
  }
  .dot {
    background: var(--attention);
  }
  .mod {
    border: 2px solid var(--fg);
  }
  .chip {
    margin-right: var(--s-1);
    padding: 0 var(--s-1);
    border: 1px solid var(--border);
    border-radius: var(--r-s);
    color: var(--muted);
    font-size: var(--fs-s);
    line-height: 1.3;
  }
  .mark {
    display: inline-flex;
    margin-right: var(--s-1);
    color: var(--muted);
  }
</style>
