<script lang="ts">
  // The one-letter change badge: M modified, A new, D deleted, R renamed, T type changed, ? untracked, U conflict.
  let { letter }: { letter: string } = $props()
</script>

<span class="badge {letter}" aria-hidden="true">{letter}</span>

<style>
  .badge {
    flex: none;
    display: grid;
    place-items: center;
    width: 18px;
    height: 18px;
    border-radius: var(--r-s);
    background: var(--hover);
    color: var(--muted);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    line-height: 1;
  }
  .M,
  .T,
  .R {
    background: var(--hover);
    color: var(--fg);
  }
  .A {
    background: var(--diff-add);
    color: var(--success);
  }
  .D,
  .U {
    background: var(--diff-del);
    color: var(--danger);
  }
</style>
