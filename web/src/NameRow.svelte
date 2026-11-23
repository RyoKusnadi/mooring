<script lang="ts">
  import { composing } from './lib/keys'
  import { onMount, tick } from 'svelte'
  import { nameProblem } from './lib/paths'

  let { depth, label, initial, onDone, onCancel }: { depth: number; label: string; initial: string; onDone: (name: string) => void; onCancel: () => void } = $props()

  let value = $state('')
  let problem = $state('')
  let input: HTMLInputElement | undefined = $state()
  let finished = false

  onMount(async () => {
    value = initial
    await tick()
    input?.focus()
    // A rename starts with the name selected, up to the extension.
    const dot = initial.lastIndexOf('.')
    input?.setSelectionRange(0, dot > 0 ? dot : initial.length)
  })

  function commit() {
    if (finished) return
    const name = value.trim()
    if (initial === '' && name === '') return cancel()
    const p = nameProblem(name)
    if (p) return void (problem = p)
    finished = true
    onDone(name)
  }

  function cancel() {
    if (finished) return
    finished = true
    onCancel()
  }
</script>

<div class="name" style:--depth={depth}>
  <input
    class="input mono"
    bind:this={input}
    bind:value
    aria-label={label}
    aria-invalid={problem ? 'true' : undefined}
    aria-describedby={problem ? 'name-problem' : undefined}
    autocomplete="off"
    spellcheck="false"
    oninput={() => (problem = '')}
    onblur={() => (problem ? cancel() : commit())}
    onkeydown={(e) => {
      if (e.key === 'Enter' && !composing(e)) commit()
      if (e.key === 'Escape') {
        e.stopPropagation()
        cancel()
      }
    }}
  />
  {#if problem}<p id="name-problem" class="error" role="alert">{problem}</p>{/if}
</div>

<style>
  .name {
    padding: 2px var(--s-1) 2px calc(var(--s-1) + var(--depth, 0) * var(--s-4));
  }
  .error {
    margin-top: var(--s-1);
  }
</style>
