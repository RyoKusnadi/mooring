<script lang="ts">
  // The lanes of one history row: lines that run through, come in from above or leave below, and the commit's dot.
  // A merge is a ring and a plain commit is a filled dot, so the difference never rests on colour.
  import type { GraphRow } from './lib/graph'

  let { row, lanes }: { row: GraphRow; lanes: number } = $props()

  const LANE = 14
  // Lanes past the last drawn one share its column.
  const x = (lane: number) => Math.min(lane, lanes - 1) * LANE + LANE / 2
  const stroke = (color: number) => `var(--lane-${color % 6})`
</script>

<span class="lane" style:width="{lanes * LANE}px">
<svg class="graph" width={lanes * LANE} aria-hidden="true" focusable="false">
  {#each row.through as e}
    <line x1={x(e.lane)} y1="0" x2={x(e.lane)} y2="100%" stroke={stroke(e.color)} />
  {/each}
  {#each row.up as e}
    <line x1={x(e.lane)} y1="0" x2={x(row.col)} y2="50%" stroke={stroke(e.color)} />
  {/each}
  {#each row.down as e}
    <line x1={x(row.col)} y1="50%" x2={x(e.lane)} y2="100%" stroke={stroke(e.color)} />
  {/each}
  {#if row.merge}
    <circle cx={x(row.col)} cy="50%" r="4" fill="var(--bg)" stroke={stroke(row.color)} stroke-width="2" />
  {:else}
    <circle cx={x(row.col)} cy="50%" r="4" fill={stroke(row.color)} stroke="var(--bg)" stroke-width="1" />
  {/if}
</svg>
</span>

<style>
  .lane {
    position: relative;
    display: block;
    flex: none;
    align-self: stretch;
  }
  .graph {
    position: absolute;
    inset: 0;
    display: block;
    width: 100%;
    height: 100%;
    stroke-width: 2;
    --lane-0: var(--accent);
    --lane-1: var(--ansi-4);
    --lane-2: var(--ansi-5);
    --lane-3: var(--ansi-3);
    --lane-4: var(--ansi-6);
    --lane-5: var(--ansi-2);
  }
  circle {
    stroke-linecap: round;
  }
</style>
