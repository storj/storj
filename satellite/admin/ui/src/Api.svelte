<script lang="ts">
  import type { Operation } from "./ui-generator";
  import { Admin } from "./api";

  import UIGen from "./UIGenerator.svelte";

  const baseURL = `${window.location.protocol}//${window.location.host}/api`;
  let api: Admin;
  let selectGroup: HTMLSelectElement;
  let selectedGroupOp: Operation[];
  let selectedOp: Operation;
  let authToken: string;

  function confirmAuthToken() {
    if (authToken) {
      api = new Admin(baseURL, authToken);
    } else {
      api = null;
    }
  }

  $: {
    if (selectGroup) selectGroup.focus();
  }
</script>

<p>
  In order to use the API you have to set the authentication token in the input
  box and press enter or click the "confirm" button.
</p>
<p>
  Token: <input
    bind:value={authToken}
    on:focus={() => (api = null)}
    on:keyup={(e) => {
      if (e.keyCode === 13) confirmAuthToken();
    }}
    type="password"
    size="48"
  />
  <button on:click={confirmAuthToken}>confirm</button>
</p>

{#if api}
  <p>
    Operation:
    <select bind:value={selectedGroupOp} bind:this={selectGroup}>
      <option selected />
      {#each Object.keys(api.operations) as group}
        <option value={api.operations[group]}>{group}</option>
      {/each}
    </select>
    {#if selectedGroupOp}
      <select bind:value={selectedOp}>
        <option selected />
        {#each selectedGroupOp as op}
          <option value={op}>{op.name}</option>
        {/each}
      </select>
    {/if}
  </p>
  <hr />
  <p>
    {#if selectedOp}
      <UIGen operation={selectedOp} />
    {/if}
  </p>
{/if}
