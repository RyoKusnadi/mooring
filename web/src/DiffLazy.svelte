<script lang="ts">
  import CommentBar from './CommentBar.svelte'
  // The diff view loads on first use, like the editor, so the app shell stays small.
  let { id, folder, path, staged, commit = '', loading = false, onflip }: { id: string; folder: string; path: string; staged: boolean; commit?: string; loading?: boolean; onflip?: (staged: boolean) => void } = $props()
</script>

<div class="wrap">
  <div class="main">
    {#await import('./DiffView.svelte')}
      <p role="status">Loading the diff…</p>
    {:then view}
      <view.default {id} {folder} {path} {staged} {commit} {loading} {onflip} />
    {:catch}
      <p>The diff view could not load. Reload the page and try again.</p>
    {/await}
  </div>
  <CommentBar />
</div>

<style>
  .wrap {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .main {
    flex: 1;
    min-height: 0;
  }
  p {
    margin: 0;
    padding: var(--s-5) var(--s-4);
    color: var(--muted);
  }
</style>
