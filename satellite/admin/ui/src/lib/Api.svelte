<!--
Copyright (C) 2021 Storj Labs, Inc.
See LICENSE for copying information.

This component is the glue between the Admin API client and the "UIGenerator"
component.

It list all the operations that the API exposes and render the Web UI, through
the `UIGenerator.svelte` component, when the user selects the operation that
wants to perform.
-->
<script lang="ts">
	import type { Operation } from '$lib/ui-generator';
	import { Admin } from '$lib/api';

	import UIGen from '$lib/UIGenerator.svelte';

	const baseURL = `${window.location.protocol}//${window.location.host}/api`;
	let api: Admin = new Admin(baseURL);
	let selectedGroupOp: Operation[];
	let selectedOp: Operation;
	let authToken: string;

	function confirmAuthToken() {
		if (authToken) {
			api = new Admin(baseURL, authToken);
		} else {
			api = new Admin(baseURL);
		}
	}
</script>

<p>
	If you did not log in using Oauth (e.g. with Google), you have to set the authentication token in
	the input box and press enter or click the "confirm" button.
</p>
<p>
	Token: <input
		bind:value={authToken}
		on:focus={() => {
			// This allows to select the empty item of the second select.
			selectedOp = null;
		}}
		on:keyup={(e) => {
			if (e.key.toLowerCase() === 'enter') confirmAuthToken();
		}}
		type="password"
		size="48"
	/>
	<button on:click={confirmAuthToken}>confirm</button>
</p>
<p>
	Operation:
	<select
		bind:value={selectedGroupOp}
		on:change={() => {
			// This allows hiding the UIGen component when this select change until
			// a new operations is selected in the following select element and also
			// selecting the empty item of the select.
			selectedOp = null;
		}}
	>
		<option selected />
		{#each Object.keys(api.operations) as group}
			<option value={api.operations[group]}>{group}</option>
		{/each}
	</select>
	{#if selectedGroupOp}
		<select bind:value={selectedOp}>
			<option selected />
			{#each selectedGroupOp as op (op)}
				<option value={op}>{op.name}</option>
			{/each}
		</select>
	{/if}
</p>
<hr />
<p>
	{#if selectedOp}
		{#key selectedOp}
			<UIGen operation={selectedOp} />
		{/key}
	{/if}
</p>
