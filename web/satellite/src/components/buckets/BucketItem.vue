// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<label v-bind:class="checked ? 'container selected' : 'container'">
		<div>
			<input type="checkbox" v-model="checked" @change="onChange">
			<span v-bind:class="[isCheckboxError ? 'checkmark error': 'checkmark']"></span>
		</div>
		<p class="container__item">test</p>
		<p class="container__item">test</p>
		<p class="container__item">test</p>
		<p class="container__item">test</p>
		<p class="container__item">test</p>
	</label>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';

    @Component(
        {
            data: () => {
                return {
                    checked: false
                };
            },
            methods: {
                // Emits data to parent component
                onChange() {
                    this.$emit('setData', this.$data.checked);
                }
            },
            props: {
                isCheckboxError: {
                    type: Boolean,
                    default: false
                },
            },
        }
    )
    export default class BucketItem extends Vue {}
</script>

<style scoped lang="scss">
	.container {
		display: grid;
		grid-template-columns: 2% 20% 30% 20% 20% 8%;
		cursor: pointer;
		position: relative;
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
			font-family: 'montserrat_medium';
			font-size: 16px;
			margin: 0;
		}
	}
	.container.selected {
		background: #2379EC;
		box-shadow: 0px 6px 20px rgba(39, 132, 255, 0.4);
		p {
			color: #fff;
		}
	}
	.container input {
		position: absolute;
		opacity: 0;
		cursor: pointer;
		height: 0;
		width: 0;
	}
	.checkmark {
		position: absolute;
		left: 25px;
		top: 50%;
		transform: translateY(-50%);
		height: 25px;
		width: 25px;
		border: 1px solid rgba(56, 75, 101, 0.4);
		border-radius: 4px;
	}
	.container:hover input ~ .checkmark {
		background-color: #fff;
	}
	.container input:checked ~ .checkmark {
		border: 1px solid #fff;
		background-color: #fff;
	}
	.checkmark:after {
		content: "";
		position: absolute;
		display: none;
	}
	.checkmark.error {
		border-color: red;
	}
	.container input:checked ~ .checkmark:after {
		display: block;
	}
	.container .checkmark:after {
		left: 9px;
		top: 5px;
		width: 5px;
		height: 10px;
		border: solid #2379EC;
		border-width: 0 3px 3px 0;
		-webkit-transform: rotate(45deg);
		-ms-transform: rotate(45deg);
		transform: rotate(45deg);
	}
	@media screen and (max-width: 1600px) {
		.container {
			grid-template-columns: 2% 20% 30% 20% 15% 13%;
			padding: 20px 0px 20px 70px;
		}
	}
</style>
