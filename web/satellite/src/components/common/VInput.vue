// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="input-container" :aria-roledescription="roleDescription">
        <div v-if="!isOptional" class="label-container">
            <div class="label-container__main">
                <ErrorIcon v-if="error" class="label-container__error-icon" />
                <h3 v-if="!error" class="label-container__main__label">
                    <span>{{ label }}</span>
                    <span class="add-label">{{ additionalLabel }}</span>
                </h3>
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
            class="textarea"
            :placeholder="placeholder"
            :style="style.inputStyle"
            :rows="5"
            :cols="40"
            :maxlength="maxSymbols"
            :disabled="disabled"
            :autocomplete="autocomplete"
            wrap="hard"
            @input="onInput"
            @change="onInput"
        />
        <input
            v-if="!isMultiline"
            :id="label"
            v-model="value"
            class="input"
            :class="{'password-input' : isPassword}"
            :placeholder="placeholder"
            :type="type"
            :style="style.inputStyle"
            :maxlength="maxSymbols"
            :disabled="disabled"
            :autocomplete="autocomplete"
            @input="onInput"
            @change="onInput"
            @focus="showPasswordStrength"
            @blur="hidePasswordStrength"
        >

        <!--2 conditions of eye image (crossed or not) -->
        <PasswordHiddenIcon
            v-if="isPasswordHiddenState"
            class="input-container__image"
            @click="changeVision"
        />
        <PasswordShownIcon
            v-if="isPasswordShownState"
            class="input-container__image"
            @click="changeVision"
        />
        <!-- end of image-->
    </div>
</template>

<script setup lang="ts">

import { computed, onBeforeMount, ref, watch } from 'vue';

import ErrorIcon from '@/../static/images/register/ErrorInfo.svg';
import PasswordShownIcon from '@/../static/images/common/passwordShown.svg';
import PasswordHiddenIcon from '@/../static/images/common/passwordHidden.svg';

const textType = 'text';
const passwordType = 'password';

const props = withDefaults(defineProps<{
    additionalLabel?: string,
    initValue?: string,
    label?: string,
    height?: string,
    width?: string,
    error?: string,
    placeholder?: string,
    roleDescription?: string,
    currentLimit?: number,
    maxSymbols?: number,
    isOptional?: boolean,
    isLimitShown?: boolean,
    isMultiline?: boolean,
    isLoading?: boolean,
    isPassword?: boolean,
    isWhite?: boolean,
    withIcon?: boolean,
    disabled?: boolean,
    autocomplete?: string,
}>(), {
    additionalLabel: '',
    initValue: '',
    placeholder: '',
    label: '',
    error: '',
    roleDescription: 'input-container',
    height: '48px',
    width: '100%',
    currentLimit: 0,
    maxSymbols: Number.MAX_SAFE_INTEGER,
    isOptional: false,
    isLimitShown: false,
    isLoading: false,
    isPassword: false,
    isWhite: false,
    withIcon: false,
    disabled: false,
    autocomplete: 'off',
});

const emit = defineEmits(['showPasswordStrength', 'hidePasswordStrength', 'setData']);

const value = ref('');
const isPasswordShown = ref(false);
const type = ref(textType);

const isPasswordHiddenState = computed(() => {
    return props.isPassword && !isPasswordShown.value;
});

const isPasswordShownState = computed(() => {
    return props.isPassword && isPasswordShown.value;
});

/**
 * Returns style objects depends on props.
 */
const style = computed(() => {
    return {
        inputStyle: {
            width: props.width,
            height: props.height,
            padding: props.withIcon ? '0 30px 0 50px' : '',
        },
        labelStyle: {
            color: props.isWhite ? 'white' : '#354049',
        },
        errorStyle: {
            color: props.isWhite ? 'white' : '#FF5560',
        },
    };
});

function showPasswordStrength(): void {
    emit('showPasswordStrength');
}

function hidePasswordStrength(): void {
    emit('hidePasswordStrength');
}

/**
 * triggers on input.
 */
function onInput(event: Event): void {
    const target = event.target as HTMLInputElement;
    value.value = target.value;

    emit('setData', target.value);
}

/**
 * Triggers input type between text and password to show/hide symbols.
 */
function changeVision(): void {
    isPasswordShown.value = !isPasswordShown.value;
    type.value = isPasswordShown.value ? textType : passwordType;
}

watch(() => props.initValue, (val, oldVal) => {
    if (val === oldVal) return;
    value.value = val;
});

onBeforeMount(() => {
    type.value = props.isPassword ? passwordType : textType;

    if (props.initValue) {
        value.value = props.initValue;
        emit('setData', props.initValue);
    }
});
</script>

<style scoped lang="scss">
    .input-container {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        margin-top: 20px;
        width: 100%;
        font-family: 'font_regular', sans-serif;
        position: relative;

        &__image {
            position: absolute;
            right: 25px;
            bottom: 5px;
            transform: translateY(-50%);
            z-index: 20;
            cursor: pointer;

            &:hover .input-container__image__path {
                fill: #2683ff !important;
            }
        }
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
            width: 100%;
            display: flex;
            justify-content: flex-start;
            align-items: center;

            &__label {
                width: 100%;
                display: flex;
                justify-content: space-between;
                align-items: center;
                font-size: 15px;
                color: #354049;

                & .add-label {
                    font-size: 12px;
                    line-height: 18px;
                    color: var(--c-grey-5) !important;
                }
            }

            &__error {
                font-size: 16px;
                line-height: 21px;
                color: #ff5560;
                margin-left: 10px;
            }
        }

        &__limit {
            margin-left: 5px;
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

    .input,
    .textarea {
        font-size: 15px;
        resize: none;
        height: 52px;
        width: 100%;
        padding: 0;
        text-indent: 16px;
        border-color: #ccc;
        border-radius: 6px;
        box-shadow: none;
        box-sizing: border-box;
        transition: border-color 50ms ease-in-out;

        &:hover {
            border-color: var(--c-blue-6);
        }

        &::placeholder {
            opacity: 0.6;
        }
    }

    .textarea {
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

    .password-input {
        padding-right: 55px;
    }

    @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
    }
</style>
