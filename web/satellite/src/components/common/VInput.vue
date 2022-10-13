// Copyright (C) 2022 Storj Labs, Inc.
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
import { Component, Prop, Vue } from 'vue-property-decorator';

import PasswordHiddenIcon from '@/../static/images/common/passwordHidden.svg';
import PasswordShownIcon from '@/../static/images/common/passwordShown.svg';
import ErrorIcon from '@/../static/images/register/ErrorInfo.svg';

// @vue/component
@Component({
    components: {
        ErrorIcon,
        PasswordHiddenIcon,
        PasswordShownIcon,
    },
})
// TODO: merge these two components to have one single source of truth.
export default class VInput extends Vue {
    @Prop({ default: '' })
    private readonly additionalLabel: string;
    @Prop({ default: 0 })
    private readonly currentLimit: number;
    @Prop({ default: false })
    private readonly isOptional: boolean;
    @Prop({ default: false })
    private readonly isLimitShown: boolean;
    @Prop({ default: false })
    private readonly isMultiline: boolean;
    @Prop({ default: false })
    private readonly isLoading: boolean;
    @Prop({ default: '' })
    protected readonly initValue: string;
    @Prop({ default: '' })
    protected readonly label: string;
    @Prop({ default: 'default' })
    protected readonly placeholder: string;
    @Prop({ default: false })
    protected readonly isPassword: boolean;
    @Prop({ default: '48px' })
    protected readonly height: string;
    @Prop({ default: '100%' })
    protected readonly width: string;
    @Prop({ default: '' })
    protected readonly error: string;
    @Prop({ default: Number.MAX_SAFE_INTEGER })
    protected readonly maxSymbols: number;
    @Prop({ default: false })
    private readonly isWhite: boolean;
    @Prop({ default: false })
    private readonly withIcon: boolean;
    @Prop({ default: false })
    private readonly disabled: boolean;
    @Prop({ default: 'input-container' })
    private readonly roleDescription: boolean;

    private readonly textType: string = 'text';
    private readonly passwordType: string = 'password';

    private type: string = this.textType;
    private isPasswordShown = false;

    public value: string;

    public created() {
        this.type = this.isPassword ? this.passwordType : this.textType;
        this.value = this.initValue;
    }

    public showPasswordStrength(): void {
        this.$emit('showPasswordStrength');
    }

    public hidePasswordStrength(): void {
        this.$emit('hidePasswordStrength');
    }

    /**
     * triggers on input.
     */
    public onInput(event: Event): void {
        const target = event.target as HTMLInputElement;
        this.value = target.value;

        this.$emit('setData', this.value);
    }

    /**
     * Triggers input type between text and password to show/hide symbols.
     */
    public changeVision(): void {
        this.isPasswordShown = !this.isPasswordShown;
        this.type = this.isPasswordShown ? this.textType : this.passwordType;
    }

    public get isPasswordHiddenState(): boolean {
        return this.isPassword && !this.isPasswordShown;
    }

    public get isPasswordShownState(): boolean {
        return this.isPassword && this.isPasswordShown;
    }

    /**
     * Returns style objects depends on props.
     */
    protected get style(): Record<string, unknown> {
        return {
            inputStyle: {
                width: this.width,
                height: this.height,
                padding: this.withIcon ? '0 30px 0 50px' : '',
            },
            labelStyle: {
                color: this.isWhite ? 'white' : '#354049',
            },
            errorStyle: {
                color: this.isWhite ? 'white' : '#FF5560',
            },
        };
    }
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
