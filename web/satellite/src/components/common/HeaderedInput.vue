// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="input-container">
		<div v-if="!isOptional" class="label-container">
			<img v-if="error" src="../../../static/images/register/ErrorInfo.svg"/>
			<h3 v-if="!error">{{label}}</h3>
			<h3  v-if="!error" class="label-container__add-label">{{additionalLabel}}</h3>
			<h3 class="label-container__error" v-if="error">{{error}}</h3>
		</div>
		<div v-if="isOptional" class="optional-label-container">
			<h3>{{label}}</h3>
			<h4>Optional</h4>
		</div>
		<textarea
            v-if="isMultiline"
            :id="this.$props.label"
            :placeholder="this.$props.placeholder"
            :style="style"
            :rows="5"
            :cols="40"
            wrap="hard"
			v-model.lazy="value"
			@change="onInput"
            @input="onInput">
		</textarea>
		<input
            v-if="!isMultiline"
            :id="this.$props.label"
            :placeholder="this.$props.placeholder"
            v-bind:type="[isPassword ? 'password': 'text']"
            v-model.lazy="value"
            @change="onInput"
            @input="onInput"
            :style="style"/>
	</div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// Custom input component with labeled header
@Component(
    {
        data: function () {
            return {
                value: this.$props.initValue ? this.$props.initValue : '',
            };
        },
        methods: {
            // Emits data to parent component
            onInput() {
                this.$emit('setData', this.$data.value);
            },
            setValue(value: string) {
                this.$data.value = value;
            }
        },
        props: {
            initValue: {
                type: String,
            },
            label: {
                type: String,
                default: ''
            },
            additionalLabel: {
                type: String,
                default: ''
            },
            error: {
                type: String
            },
            placeholder: {
                type: String,
                default: 'default'
            },
            isOptional: {
                type: Boolean,
                default: false
            },
            isMultiline: {
                type: Boolean,
                default: false
            },
            isPassword: {
                type: Boolean,
                default: false
            },
            height: {
                type: String,
                default: '48px'
            },
            width: {
                type: String,
                default: '100%'
            },
        },
        computed: {
            style: function () {
                return {width: this.$props.width, height: this.$props.height};
            },
        },
    },
)

export default class HeaderedInput extends Vue {}

</script>

<style scoped lang="scss">
	.input-container {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		margin-top: 10px;
		width: 48%;
	}
	.label-container {
		display: flex;
		justify-content: flex-start;
		flex-direction: row;

		&__add-label {
			margin-left: 5px;
			font-family: 'font_regular';
			font-size: 16px;
			line-height: 21px;
			color: rgba(56, 75, 101, 0.4);
		}

		&__error {
			color: #FF5560;
			margin-left: 10px;
		}
	}
	.optional-label-container {
		display: flex;
		flex-direction: row;
		justify-content: space-between;
		align-items: center;
		width: 100%;
		h4 {
			font-family: 'font_regular';
			font-size: 16px;
			line-height: 21px;
			color: #AFB7C1;
		}
	}
	input,
	textarea {
		font-family: 'font_regular';
		font-size: 16px;
		line-height: 21px;
		resize: none;
		height: 48px;
		width: 100%;
		text-indent: 20px;
		border-color: rgba(56, 75, 101, 0.4);
		border-radius: 6px;
		outline: none;
		box-shadow: none;
	}
	textarea {
		padding-top: 20px;
	}
	h3 {
		font-family: 'font_regular';
		font-size: 16px;
		line-height: 21px;
		color: #354049;
	}
</style>
