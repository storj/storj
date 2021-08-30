<script lang="ts">
  import { prettyPrintJson as prettyJSON } from "pretty-print-json";
  import type { Operation } from "./ui-generator";
  import { InputText, Select, Textarea } from "./ui-generator";

  import UIInputText from "./UIGeneratorInputText.svelte";
  import UISelect from "./UIGeneratorSelect.svelte";
  import UITextarea from "./UIGeneratorTextarea.svelte";

  type opArg = boolean | number | string;

  function execOperation(op: Operation, args: opArg[]) {
    result = op.func(...args).then((data) => {
      form.reset();
      return data;
    });
  }

  export let operation: Operation;
  let opArgs: any[] = new Array(operation.params.length);
  let result: Promise<object | null>;
  let form: HTMLFormElement;
</script>

<div>
  <p>{operation.desc}</p>
  <form
    bind:this={form}
    on:submit|preventDefault={() => execOperation(operation, opArgs)}
  >
    {#each operation.params as param, i}
      <br />
      {#if param[1] instanceof InputText}
        <UIInputText
          label={param[0]}
          config={param[1]}
          bind:value={opArgs[i]}
        />
      {:else if param[1] instanceof Select}
        <UISelect label={param[0]} config={param[1]} bind:value={opArgs[i]} />
      {:else if param[1] instanceof Textarea}
        <UITextarea label={param[0]} config={param[1]} bind:value={opArgs[i]} />
      {/if}
    {/each}
    <br />
    <input type="submit" value="submit" />
  </form>
</div>
<output>
  {#if result !== undefined}
    {#await result}
      <p>Sending...</p>
    {:then data}
      <p class="successful">
        <b>Operation successful</b>
        {#if data != null}
          <br /><br />
          HTTP Response body:
          <pre>{@html prettyJSON.toHtml(data)}</pre>
        {/if}
      </p>
    {:catch err}
      <p class="failure">
        <b>Operation failed</b>
        <br /><br />
        {err.name}: {err.message}
        {#if err.responseStatusCode}
          <br />
          HTTP Response status code: {err.responseStatusCode}
          {#if err.responseBody}
            <br />
            HTTP Response body:
            <pre>{@html prettyJSON.toHtml(err.responseBody)}</pre>
          {/if}
        {/if}
      </p>
    {/await}
  {/if}
</output>

<style>
  .failure b {
    color: red;
    text-decoration: underline;
  }

  .successful b {
    text-decoration: underline;
  }
</style>
