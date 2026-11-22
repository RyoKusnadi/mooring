<script lang="ts">
  let { matchCase = $bindable(), regex = $bindable(), glob = $bindable(), disabled = false, onChange }: { matchCase: boolean; regex: boolean; glob: string; disabled?: boolean; onChange: () => void } = $props()
</script>

<div class="opts">
  <label><input type="checkbox" {disabled} bind:checked={matchCase} onchange={onChange} /> Match case</label>
  <label><input type="checkbox" {disabled} bind:checked={regex} onchange={onChange} /> Regex</label>
  <label class="glob">Files <input class="input mono" type="text" {disabled} placeholder="*.go or !*.test.ts" aria-label="Only files matching" spellcheck="false" bind:value={glob} oninput={onChange} /></label>
</div>

<style>
  .opts {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-3);
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
    font-size: var(--fs-s);
  }
  .glob {
    display: flex;
    flex: 1 1 14ch;
    align-items: center;
    gap: var(--s-2);
  }
  @media (pointer: coarse) {
    .glob {
      flex-basis: 100%;
    }
    label {
      display: flex;
      align-items: center;
      min-height: 44px;
    }
  }
</style>
