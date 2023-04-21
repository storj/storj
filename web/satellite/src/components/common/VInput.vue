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

<script lang="ts">

import { computed, defineComponent, onBeforeMount, ref } from 'vue';

import ErrorIcon from '@/../static/images/register/ErrorInfo.svg';
import PasswordShownIcon from '@/../static/images/common/passwordShown.svg';
import PasswordHiddenIcon from '@/../static/images/common/passwordHidden.svg';

const textType = 'text';
const passwordType = 'password';

export default defineComponent({
    name: 'VInput',
    components: {
        PasswordHiddenIcon,
        PasswordShownIcon,
        ErrorIcon,
    },
    props:  {
        additionalLabel: {
            type: String,
            default: '',
        },
        currentLimit: {
            type: Number,
            default: 0,
        },
        isOptional: Boolean,
        isLimitShown: Boolean,
        isMultiline: Boolean,
        isLoading: Boolean,
        initValue: {
            type: String,
            default: '',
        },
        label: {
            type: String,
            default: '',
        },
        placeholder: {
            type: String,
            default: 'default',
        },
        isPassword: Boolean,
        height: {
            type: String,
            default: '48px',
        },
        width: {
            type: String,
            default: '100%',
        },
        error: {
            type: String,
            default: '',
        },
        maxSymbols: {
            type: Number,
            default: Number.MAX_SAFE_INTEGER,
        },
        isWhite: Boolean,
        withIcon: Boolean,
        disabled: Boolean,
        roleDescription: {
            type: String,
            default: 'input-container',
        },
    },
    emits: ['showPasswordStrength', 'hidePasswordStrength', 'setData'],
    setup(props, ctx) {
        const value = ref('');
        const isPasswordShown = ref(false);
        const type = ref(textType);

        onBeforeMount(() => {
            type.value = props.isPassword ? passwordType : textType;
            value.value = props.initValue;
        });
        return {
            isPasswordHiddenState: computed(() => {
                return props.isPassword && !isPasswordShown.value;
            }),
            isPasswordShownState: computed(() => {
                return props.isPassword && isPasswordShown.value;
            }),
            /**
             * Returns style objects depends on props.
             */
            style: computed(() => {
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
            }),
            showPasswordStrength(): void {
                ctx.emit('showPasswordStrength');
            },
            hidePasswordStrength(): void {
                ctx.emit('hidePasswordStrength');
            },
            /**
             * triggers on input.
             */
            onInput(event: Event): void {
                const target = event.target as HTMLInputElement;
                value.value = target.value;

                ctx.emit('setData', value.value);
            },
            /**
             * Triggers input type between text and password to show/hide symbols.
             */
            changeVision(): void {
                isPasswordShown.value = !isPasswordShown.value;
                type.value = isPasswordShown.value ? textType : passwordType;
            },
            value,
            isPasswordShown,
            type,
        };
    },
});
</script>

<style scoped lang="scss">
    .input-container {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        margin-top: 10px;
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
                font-size: 16px;
                line-height: 21px;
                color: #354049;

                & .add-label {
                    font-size: 14px;
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
