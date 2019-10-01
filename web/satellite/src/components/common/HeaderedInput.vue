// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="input-container">
        <div v-if="!isOptional" class="label-container">
            <img v-if="error" src="../../../static/images/register/ErrorInfo.svg"/>
            <h3 v-if="!error" class="label-container__label">{{label}}</h3>
            <h3 v-if="!error" class="label-container__label add-label">{{additionalLabel}}</h3>
            <h3 class="label-container__error" v-if="error">{{error}}</h3>
        </div>
        <div v-if="isOptional" class="optional-label-container">
            <h3 class="label-container__label">{{label}}</h3>
            <h4 class="optional-label-container__optional">Optional</h4>
        </div>
        <textarea
            class="headered-textarea"
            v-if="isMultiline"
            :id="this.label"
            :placeholder="this.placeholder"
            :style="style.inputStyle"
            :rows="5"
            :cols="40"
            wrap="hard"
            @input="onInput"
            @change="onInput"
            v-model="value">
        </textarea>
        <input
            class="headered-input"
            v-if="!isMultiline"
            :id="this.label"
            :placeholder="this.$props.placeholder"
            :type="[isPassword ? 'password': 'text']"
            @input="onInput"
            @change="onInput"
            v-model="value"
            :style="style.inputStyle"/>
    </div>
</template>

<script lang="ts">
import { Component, Prop } from 'vue-property-decorator';

import HeaderlessInput from './HeaderlessInput.vue';

// Custom input component with labeled header
@Component
export default class HeaderedInput extends HeaderlessInput {
    @Prop({default: ''})
    private readonly initValue: string;
    @Prop({default: ''})
    private readonly additionalLabel: string;
    @Prop({default: false})
    private readonly isOptional: boolean;
    @Prop({default: false})
    private readonly isMultiline: boolean;

    public constructor() {
        super();

        this.value = this.initValue;
    }
}
</script>

<style scoped lang="scss">
    .input-container {
    	display: flex;
    	flex-direction: column;
    	align-items: flex-start;
    	margin-top: 10px;
    	width: 48%;
        font-family: 'font_regular';
    }
    
    .label-container {
    	display: flex;
    	justify-content: flex-start;
        align-items: center;

        &__label {
            font-size: 16px;
            line-height: 21px;
            color: #354049;
    	}
    
    	&__error {
            font-size: 16px;
            line-height: 21px;
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
    
    	&__optional {
            font-size: 16px;
            line-height: 21px;
            color: #AFB7C1;
    	}
    }
    
    .headered-input,
    .headered-textarea {
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

    .headered-textarea {
    	padding-top: 20px;
    }

    .add-label {
        margin-left: 5px;
        color: rgba(56, 75, 101, 0.4);
    }
</style>
