<script lang="ts">
  // The "Attach image" button of the message box and the file picker it opens.
  import { IMAGE_ACCEPT } from './lib/images'

  let { disabled, describedby, onPick }: { disabled: boolean; describedby?: string; onPick: (files: File[]) => void } = $props()
  let input: HTMLInputElement | undefined = $state()
</script>

<button type="button" class="icon-btn" aria-label="Attach image" title="Attach image" {disabled} aria-describedby={describedby} onclick={() => input?.click()}>
  <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="2" y="3" width="12" height="10" rx="2" /><circle cx="6" cy="6.5" r="1" /><path d="M2.5 12l3.5-3.5 2.5 2.5 2-2 3 3" /></svg>
</button>
<input
  bind:this={input}
  type="file"
  hidden
  accept={IMAGE_ACCEPT}
  multiple
  onchange={(e) => {
    onPick([...(e.currentTarget.files ?? [])])
    e.currentTarget.value = ''
  }}
/>
