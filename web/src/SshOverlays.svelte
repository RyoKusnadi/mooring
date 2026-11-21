<script lang="ts">
  // Everything SSH puts on top of the workspace. Loads on first use (see Shell.svelte).
  import SshHosts from './SshHosts.svelte'
  import SshProblem from './SshProblem.svelte'
  import SshPrompt from './SshPrompt.svelte'
  import { ssh } from './lib/sshstore.svelte'

  const prompt = $derived(ssh.prompts[0])
</script>

<div data-ssh-dialogs hidden></div>
{#if ssh.dialog}<SshHosts onClose={() => (ssh.dialog = false)} />{/if}
{#if ssh.failure}{#key ssh.failure.hostId + ssh.failure.message}<SshProblem failure={ssh.failure} />{/key}{/if}
{#if prompt}{#key prompt.promptId}<SshPrompt {prompt} />{/key}{/if}
