// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<tr class="container">
		<td class="container__item">test</td>
		<td class="container__item">test</td>
		<td class="container__item">test</td>
		<td class="container__item">test</td>
		<td class="container__item">test</td>
	</tr>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';

    @Component({})

    export default class BucketItem extends Vue {}
</script>

<style scoped lang="scss">
	.container {
		transition: box-shadow .2s ease-out;
		padding: 35px 0px 35px 70px;
		transition: all .2s ease;
		margin-bottom: 10px;
		border-radius: 6px;
		-webkit-user-select: none;
		-moz-user-select: none;
		-ms-user-select: none;
		user-select: none;
		outline: none;

		&:hover {
			background: #fff;
			box-shadow: 0px 4px 4px rgba(231, 232, 238, 0.6);
		}
		&__item {
			width: 20%;
			height: 80px;
			padding-left: 30px;
			font-family: 'font_medium';
			font-size: 16px;
			margin: 0;
		}

		&:first-of-type {
			font-weight: bold;
		}
	}
	@media screen and (max-width: 1600px) {
		.container {
			grid-template-columns: 2% 20% 30% 20% 15% 13%;
			padding: 20px 0px 20px 70px;
		}
	}
</style>
