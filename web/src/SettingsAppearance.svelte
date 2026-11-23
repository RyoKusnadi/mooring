<script lang="ts">
  // Appearance section of Settings: theme and text size, kept in this browser only.
  import { SIZES, THEMES, type SizeChoice, type ThemeChoice } from './lib/appearance'
  import { appearance } from './lib/appearance.svelte'

  const THEME_NAMES: Record<ThemeChoice, string> = { system: 'Same as the system', light: 'Light', dark: 'Dark' }
  const SIZE_NAMES: Record<SizeChoice, string> = { small: 'Small', default: 'Default', large: 'Large' }
  const uid = $props.id()
</script>

<p class="help lead">These two choices are saved in this browser only. Another browser or device keeps its own.</p>

<fieldset>
  <legend>Theme</legend>
  {#each THEMES as t (t)}
    <label class="opt"><input type="radio" name="{uid}-theme" checked={appearance.theme === t} onchange={() => appearance.setTheme(t)} />{THEME_NAMES[t]}</label>
  {/each}
</fieldset>

<fieldset>
  <legend>Text size</legend>
  {#each SIZES as s (s)}
    <label class="opt"><input type="radio" name="{uid}-size" checked={appearance.size === s} onchange={() => appearance.setSize(s)} />{SIZE_NAMES[s]}</label>
  {/each}
  <p class="help">On a phone or tablet the text stays at its touch size.</p>
</fieldset>

<style>
  fieldset {
    display: grid;
    gap: var(--s-2);
    margin: 0 0 var(--s-4);
    padding: 0;
    border: 0;
  }
  legend {
    margin-bottom: var(--s-2);
    padding: 0;
    font-weight: var(--fw-semibold);
  }
  .opt {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-height: var(--control-h);
  }
  .lead {
    margin: 0 0 var(--s-4);
  }
  .help {
    margin: 0;
  }
</style>
