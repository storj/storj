<!--
Copyright (C) 2021 Storj Labs, Inc.
See LICENSE for copying information.

Children Svelte component of the `UIGenerator.svelte` component which renders
an HTML select element
-->
<script lang="ts">
	import type { Select } from '$lib/ui-generator';

	export let label: string;
	export let config: Select;
	export let value: boolean | number | string = undefined;
</script>

<!-- the empty 'for' avoids Svelte check warnings -->
<label for="">
	{label}
	{#if config.required}<sup>*</sup>{/if}:
	{#if config.multiple}
		<select bind:value required={config.required} multiple>
			{#each config.options as option}
				<option value={option.value}>{option.text}</option>
			{/each}
		</select>
	{:else}
		<select bind:value required={config.required}>
			{#each config.options as option}
				<option value={option.value}>{option.text}</option>
			{/each}
		</select>
	{/if}
</label>
