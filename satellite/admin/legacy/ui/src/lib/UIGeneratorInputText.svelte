<!--
Copyright (C) 2021 Storj Labs, Inc.
See LICENSE for copying information.

Children Svelte component of the `UIGenerator.svelte` component which renders
an HTML input element.
-->
<script lang="ts">
	import { onMount } from 'svelte';

	import type { InputText } from '$lib/ui-generator';

	export let label: string;
	export let config: InputText;
	export let value: boolean | number | string = undefined;

	// For avoiding Svelte validate errors with Typescript types, we cannot map
	// the `value` variable directly to HTML elements.
	let boolValue: boolean = undefined;
	let numValue: number = undefined;
	let strValue: string = undefined;

	// Map the initial value property when has some value to the HTML element.
	onMount(() => {
		if (value) {
			switch (typeof value) {
				case 'boolean':
					boolValue = value;
					break;
				case 'number':
					numValue = value;
					break;
				case 'string':
					strValue = value;
					break;
			}
		}
	});

	$: {
		if (boolValue !== undefined) {
			value = boolValue;
		}

		if (numValue !== undefined) {
			value = numValue;
		}

		if (strValue !== undefined) {
			value = strValue;
		}
	}
</script>

<!-- the empty 'for' avoids Svelte check warnings -->
<label for="">
	{label}
	{#if config.required}<sup>*</sup>{/if}:
	{#if config.type === 'checkbox'}
		<input type="checkbox" bind:checked={boolValue} required={config.required} />
	{:else if config.type === 'email'}
		<input type="email" bind:value={strValue} required={config.required} />
	{:else if config.type === 'number'}
		<input type="number" bind:value={numValue} required={config.required} />
	{:else if config.type === 'password'}
		<input type="password" bind:value={strValue} required={config.required} />
	{:else if config.type === 'text'}
		<input type="text" bind:value={strValue} required={config.required} />
	{:else}
		<p style="color: red;">BUG: not mapped input type: {config.type}</p>
	{/if}
</label>
