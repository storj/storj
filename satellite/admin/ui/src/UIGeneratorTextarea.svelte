<script lang="ts">
  import type { Textarea } from "./ui-generator";

  export let label: string;
  export let config: Textarea;
  export let value: string = undefined;
</script>

<label>
  <div>
    {label}{#if config.required}<sup>*</sup>{/if}:
  </div>
  <textarea required={config.required} bind:value />
</label>

<style>
  label {
    display: grid;
    row-gap: 1em;
  }
</style>
