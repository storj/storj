// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="input-wrap">
        <div class="label-container">
            <div v-if="error" class="icon">
                <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="20" height="20" rx="10" fill="#EB5757" />
                    <path d="M10.0012 11.7364C10.612 11.7364 11.1117 11.204 11.1117 10.5532V5.81218C11.1117 5.75302 11.108 5.68991 11.1006 5.63074C11.0192 5.06672 10.5565 4.62891 10.0012 4.62891C9.39037 4.62891 8.89062 5.16138 8.89062 5.81218V10.5492C8.89062 11.204 9.39037 11.7364 10.0012 11.7364Z" fill="white" />
                    <path d="M10.0001 12.8906C9.13977 12.8906 8.44531 13.5851 8.44531 14.4454C8.44531 15.3057 9.13977 16.0002 10.0001 16.0002C10.8604 16.0002 11.5548 15.3057 11.5548 14.4454C11.5583 13.5851 10.8638 12.8906 10.0001 12.8906Z" fill="white" />
                </svg>
            </div>
            <p v-if="isLabelShown" class="label-container__label" :style="style.labelStyle">{{ label }}</p>
            <p v-if="error" class="label-container__error" :style="style.errorStyle">{{ error }}</p>
        </div>
        <div
            class="headerless-input-container"
            :style="style.inputStyle"
        >
            <input
                v-model="value"
                class="headerless-input"
                :class="{'inputError' : error, 'password': isPassword}"
                :placeholder="placeholder"
                :type="type"
                @input="onInput"
                @change="onInput"
                @paste.prevent="onPaste"
                @focus="showPasswordStrength"
                @blur="hidePasswordStrength"
            >
            <!--2 conditions of eye image (crossed or not) -->
            <div
                v-if="isPasswordHiddenState"
                class="input-wrap__image icon"
                @click="changeVision"
            >
                <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path class="input-wrap__image__path" d="M10 4C4.70642 4 1 10 1 10C1 10 3.6999 16 10 16C16.3527 16 19 10 19 10C19 10 15.3472 4 10 4ZM10 13.8176C7.93537 13.8176 6.2946 12.1271 6.2946 10C6.2946 7.87285 7.93537 6.18239 10 6.18239C12.0646 6.18239 13.7054 7.87285 13.7054 10C13.7054 12.1271 12.0646 13.8176 10 13.8176Z" fill="#AFB7C1" />
                    <path d="M11.6116 9.96328C11.6116 10.8473 10.8956 11.5633 10.0116 11.5633C9.12763 11.5633 8.41162 10.8473 8.41162 9.96328C8.41162 9.07929 9.12763 8.36328 10.0116 8.36328C10.8956 8.36328 11.6116 9.07929 11.6116 9.96328Z" fill="#AFB7C1" />
                </svg>
            </div>
            <div
                v-if="isPasswordShownState"
                class="input-wrap__image icon"
                @click="changeVision"
            >
                <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path class="input-wrap__image__path" d="M10 4C4.70642 4 1 10 1 10C1 10 3.6999 16 10 16C16.3527 16 19 10 19 10C19 10 15.3472 4 10 4ZM10 13.8176C7.93537 13.8176 6.2946 12.1271 6.2946 10C6.2946 7.87285 7.93537 6.18239 10 6.18239C12.0646 6.18239 13.7054 7.87285 13.7054 10C13.7054 12.1271 12.0646 13.8176 10 13.8176Z" fill="#AFB7C1" />
                    <path d="M11.6121 9.96328C11.6121 10.8473 10.8961 11.5633 10.0121 11.5633C9.12812 11.5633 8.41211 10.8473 8.41211 9.96328C8.41211 9.07929 9.12812 8.36328 10.0121 8.36328C10.8961 8.36328 11.6121 9.07929 11.6121 9.96328Z" fill="#AFB7C1" />
                    <mask id="path-3-inside-1" fill="white">
                        <path fill-rule="evenodd" clip-rule="evenodd" d="M5 16.5L16 1L16.8155 1.57875L5.81551 17.0787L5 16.5Z" />
                    </mask>
                    <path class="input-wrap__image__path" fill-rule="evenodd" clip-rule="evenodd" d="M5 16.5L16 1L16.8155 1.57875L5.81551 17.0787L5 16.5Z" fill="white" />
                    <path class="input-wrap__image__path" d="M16 1L16.5787 0.184493L15.7632 -0.394254L15.1845 0.421253L16 1ZM5 16.5L4.18449 15.9213L3.60575 16.7368L4.42125 17.3155L5 16.5ZM16.8155 1.57875L17.631 2.15749L18.2098 1.34199L17.3943 0.76324L16.8155 1.57875ZM5.81551 17.0787L5.23676 17.8943L6.05227 18.473L6.63101 17.6575L5.81551 17.0787ZM15.1845 0.421253L4.18449 15.9213L5.81551 17.0787L16.8155 1.57875L15.1845 0.421253ZM17.3943 0.76324L16.5787 0.184493L15.4213 1.81551L16.2368 2.39425L17.3943 0.76324ZM6.63101 17.6575L17.631 2.15749L16 1L5 16.5L6.63101 17.6575ZM4.42125 17.3155L5.23676 17.8943L6.39425 16.2632L5.57875 15.6845L4.42125 17.3155Z" fill="white" mask="url(#path-3-inside-1)" />
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M5 17.5L16 2L16.8155 2.57875L5.81551 18.0787L5 17.5Z" fill="#AFB7C1" />
                </svg>
            </div>
            <!-- end of image-->
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

/**
 * Custom input component for login page.
 */
// @vue/component
@Component
export default class HeaderlessInput extends Vue {
    private readonly textType: string = 'text';
    private readonly passwordType: string = 'password';

    private type: string = this.textType;
    private isPasswordShown = false;

    protected value = '';

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

    public created() {
        this.type = this.isPassword ? this.passwordType : this.textType;
    }

    /**
     * Used to set default value from parent component.
     * @param value
     */
    public setValue(value: string): void {
        this.value = value;
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
        if (!event.target) { return; }
        const target = event.target as HTMLInputElement;
        if (!target || !target.value) { return; }

        if (target.value.length > this.maxSymbols) {
            this.value = target.value.slice(0, this.maxSymbols);
        } else {
            this.value = target.value;
        }

        this.$emit('setData', this.value);
    }

    public onPaste(event: ClipboardEvent): void {
        if (!event || !event.clipboardData) { return; }
        const clipped: string = event.clipboardData.getData('text');

        if (clipped.length > this.maxSymbols) {
            this.value = clipped.slice(0, this.maxSymbols);
        } else {
            this.value = clipped;
        }

        this.$emit('setData', this.value);
    }

    /**
     * Triggers input type between text and password to show/hide symbols.
     */
    public changeVision(): void {
        this.isPasswordShown = !this.isPasswordShown;
        this.type = this.isPasswordShown ? this.textType : this.passwordType;
    }

    public get isLabelShown(): boolean {
        return !!(!this.error && this.label);
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
    .input-wrap {
        position: relative;
        width: 100%;
        font-family: 'font_regular', sans-serif;

        &__image {
            position: absolute;
            right: 25px;
            top: 50%;
            transform: translateY(-50%);
            z-index: 20;
            cursor: pointer;

            &:hover .input-wrap__image__path {
                fill: var(--c-primary) !important;
            }
        }
    }

    .label-container {
        display: flex;
        justify-content: flex-start;
        align-items: flex-end;
        padding-bottom: 8px;
        flex-direction: row;
        height: auto;

        &__label {
            font-size: 16px;
            line-height: 21px;
            color: var(--v-header-base);
            margin-bottom: 0;
        }

        &__add-label {
            margin-left: 5px;
            font-size: 16px;
            line-height: 21px;
            color: rgb(56 75 101 / 40%);
        }

        &__error {
            font-size: 16px;
            margin: 18px 0 0 10px;
        }
    }

    .headerless-input-container {
        position: relative;
        box-sizing: border-box;
        height: 46px;
    }

    .headerless-input {
        font-size: 16px;
        line-height: 21px;
        resize: none;
        padding: 0 30px 0 0;
        width: 100%;
        height: 100%;
        text-indent: 20px;
        border: 1px solid rgb(56 75 101 / 40%);
        border-radius: 6px;
    }

    .headerless-input::placeholder {
        color: #384b65;
        opacity: 0.4;
    }

    .inputError::placeholder {
        color: #eb5757;
        opacity: 0.4;
    }

    .error {
        color: #ff5560;
        margin-left: 10px;
    }

    .password {
        width: calc(100% - 75px) !important;
        padding-right: 75px;
    }

    .icon {
        width: 20px;
        height: 20px;
    }
</style>
