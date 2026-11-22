<script lang="ts">
  import { icons, type IconName } from './lib/icons'

  let { name }: { name: IconName } = $props()
</script>

<svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">
  <path d={icons[name]} />
</svg>

<style>
  svg {
    flex: none;
  }
</style>
