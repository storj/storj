<!--
Copyright (C) 2021 Storj Labs, Inc.
See LICENSE for copying information.

Children Svelte component of the `UIGenerator.svelte` component which renders
an HTML text area element.
-->
<script lang="ts">
	import type { Textarea } from '$lib/ui-generator';

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
