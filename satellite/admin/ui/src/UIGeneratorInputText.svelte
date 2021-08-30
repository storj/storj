<script lang="ts">
  import type { InputText } from "./ui-generator";

  export let label: string;
  export let config: InputText;
  export let value: boolean | number | string = undefined;
</script>

<label>
  {label}
  {#if config.required}<sup>*</sup>{/if}:
  {#if config.type === "checkbox"}
    <input type="checkbox" bind:value required={config.required} />
  {:else if config.type === "email"}
    <input type="email" bind:value required={config.required} />
  {:else if config.type === "number"}
    <input type="number" bind:value required={config.required} />
  {:else if config.type === "password"}
    <input type="password" bind:value required={config.required} />
  {:else if config.type === "text"}
    <input type="text" bind:value required={config.required} />
  {:else}
    <p style="color: red;">BUG: not mapped input type: {config.type}</p>
  {/if}
</label>
