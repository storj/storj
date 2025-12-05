<!--
Copyright (C) 2021 Storj Labs, Inc.
See LICENSE for copying information.

The Svelte root component for this Web App.
-->
<script lang="ts">
	import Api from '$lib/Api.svelte';
</script>

<svelte:head>
	<title>Storj Satellite Admin Console</title>
</svelte:head>

<header>
	<h1>Satellite Admin</h1>
</header>
<main>
	<Api />
</main>
<footer>
	<p>version: v1</p>
</footer>

<style>
	header {
		text-align: center;
		padding: 1em;
		max-width: 60ch;
		margin: 0 auto;
	}

	footer {
		position: fixed;
		bottom: 0;
		width: 97vw;
		display: flex;
		flex-direction: row;
		justify-content: flex-end;
	}

	h1 {
		color: #ff3e00;
		text-transform: uppercase;
		font-size: 4em;
		font-weight: 100;
	}

	@media (min-width: 640px) {
		header {
			max-width: none;
		}
	}
</style>
