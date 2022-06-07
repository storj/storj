// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="input-container" :aria-roledescription="roleDescription">
        <div v-if="!isOptional" class="label-container">
            <div class="label-container__main">
                <ErrorIcon v-if="error" class="label-container__error-icon" />
                <h3 v-if="!error" class="label-container__main__label">{{ label }}</h3>
                <h3 v-if="!error" class="label-container__main__label add-label">{{ additionalLabel }}</h3>
                <h3 v-if="error" class="label-container__main__error">{{ error }}</h3>
                <div v-if="isLoading" class="loader" />
            </div>
            <h3 v-if="isLimitShown" class="label-container__limit">{{ currentLimit }}/{{ maxSymbols }}</h3>
        </div>
        <div v-if="isOptional" class="optional-label-container">
            <h3 class="label-container__label">{{ label }}</h3>
            <h4 class="optional-label-container__optional">Optional</h4>
        </div>
        <textarea
            v-if="isMultiline"
            :id="label"
            v-model="value"
            class="headered-textarea"
            :placeholder="placeholder"
            :style="style.inputStyle"
            :rows="5"
            :cols="40"
            wrap="hard"
            :disabled="isDisabled"
            @input="onInput"
            @change="onInput"
        />
        <input
            v-if="!isMultiline"
            :id="label"
            v-model="value"
            class="headered-input"
            :placeholder="placeholder"
            :type="[isPassword ? 'password': 'text']"
            :style="style.inputStyle"
            :disabled="isDisabled"
            @input="onInput"
            @change="onInput"
        >
    </div>
</template>

<script lang="ts">
import { Component, Prop } from 'vue-property-decorator';

import ErrorIcon from '@/../static/images/register/ErrorInfo.svg';

import HeaderlessInput from './HeaderlessInput.vue';

// Custom input component with labeled header
// @vue/component
@Component({
    components: {
        ErrorIcon,
    },
})
// TODO: merge these two components to have one single source of truth.
export default class HeaderedInput extends HeaderlessInput {
    @Prop({default: ''})
    private readonly additionalLabel: string;
    @Prop({default: 0})
    private readonly currentLimit: number;
    @Prop({default: false})
    private readonly isOptional: boolean;
    @Prop({default: false})
    private readonly isLimitShown: boolean;
    @Prop({default: false})
    private readonly isMultiline: boolean;
    @Prop({default: false})
    private readonly isLoading: boolean;
}
</script>

<style scoped lang="scss">
    .input-container {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        margin-top: 10px;
        width: 100%;
        font-family: 'font_regular', sans-serif;
    }

    .label-container {
        width: 100%;
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 10px;

        &__error-icon {
            min-height: 20px;
            min-width: 20px;
        }

        &__main {
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
                color: #ff5560;
                margin-left: 10px;
            }
        }

        &__limit {
            font-size: 16px;
            line-height: 21px;
            color: rgb(56 75 101 / 40%);
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
            color: #afb7c1;
        }
    }

    .headered-input,
    .headered-textarea {
        font-size: 16px;
        line-height: 21px;
        resize: none;
        height: 48px;
        width: 100%;
        padding: 0;
        text-indent: 20px;
        border-color: rgb(56 75 101 / 40%);
        border-radius: 6px;
        outline: none;
        box-shadow: none;
        box-sizing: border-box;

        &::placeholder {
            opacity: 0.6;
        }
    }

    .headered-textarea {
        padding: 15px 22px;
        text-indent: 0;
        line-height: 26px;
    }

    .add-label {
        margin-left: 5px;
        color: rgb(56 75 101 / 40%);
    }

    .loader {
        margin-left: 10px;
        border: 5px solid #f3f3f3;
        border-top: 5px solid #3498db;
        border-radius: 50%;
        width: 15px;
        height: 15px;
        animation: spin 2s linear infinite;
    }

    @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
    }
</style>
