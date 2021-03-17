// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="generate-container">
        <h1 class="generate-container__title">Encryption Passphrase</h1>
        <div class="generate-container__warning">
            <div class="generate-container__warning__header">
                <WarningIcon/>
                <p class="generate-container__warning__header__label">Save Your Encryption Passphrase</p>
            </div>
            <p class="generate-container__warning__message">
                You’ll need this passphrase to access data in the future. This is the only time it will be displayed.
                Be sure to write it down.
            </p>
        </div>
        <div class="generate-container__choosing">
            <p class="generate-container__choosing__label">Choose Passphrase Type</p>
            <div class="generate-container__choosing__right">
                <p
                    class="generate-container__choosing__right__option left-option"
                    :class="{ active: isGenerateState }"
                    @click="onChooseGenerate"
                >
                    Generate Phrase
                </p>
                <p
                    class="generate-container__choosing__right__option"
                    :class="{ active: isCreateState }"
                    @click="onChooseCreate"
                >
                    Create Phrase
                </p>
            </div>
        </div>
        <div class="generate-container__value-area">
            <div class="generate-container__value-area__mnemonic" v-if="isGenerateState">
                <p class="generate-container__value-area__mnemonic__value">{{ passphrase }}</p>
                <VButton
                    class="generate-container__value-area__mnemonic__button"
                    label="Copy"
                    width="66px"
                    height="30px"
                    :on-press="onCopyClick"
                />
            </div>
            <div class="generate-container__value-area__password" v-else>
                <HeaderedInput
                    class="generate-container__value-area__password__input"
                    placeholder="Strong passphrases contain 12 characters or more"
                    @setData="onChangePassphrase"
                    :error="errorMessage"
                    label="Create Your Passphrase"
                />
            </div>
        </div>
        <label class="generate-container__check-area" :class="{ error: isError }" for="pass-checkbox">
            <input
                class="generate-container__check-area__checkbox"
                id="pass-checkbox"
                type="checkbox"
                v-model="isChecked"
                @change="isError = false"
            >
            Yes, I wrote this down or saved it somewhere.
        </label>
        <VButton
            class="generate-container__next-button"
            label="Next"
            width="100%"
            height="48px"
            :on-press="onProceed"
            :is-disabled="isLoading"
        />
    </div>
</template>

<script lang="ts">
import * as bip39 from 'bip39';
import { Component, Prop, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';
import WarningIcon from '@/../static/images/accessGrants/warning.svg';

@Component({
    components: {
        WarningIcon,
        BackIcon,
        VButton,
        HeaderedInput,
    },
})
export default class GeneratePassphrase extends Vue {
    @Prop({ default: () => null })
    public readonly onButtonClick: () => void;
    @Prop({ default: () => null })
    public readonly setParentPassphrase: (passphrase: string) => void;
    @Prop({ default: false })
    public readonly isLoading: boolean;

    public isGenerateState: boolean = true;
    public isCreateState: boolean = false;
    public isChecked: boolean = false;
    public isError: boolean = false;
    public passphrase: string = '';
    public errorMessage: string = '';

    /**
     * Lifecycle hook after initial render.
     * Generates mnemonic string.
     */
    public mounted(): void {
        this.passphrase = bip39.generateMnemonic();
        this.setParentPassphrase(this.passphrase);
    }

    public onProceed(): void {
        if (!this.passphrase) {
            this.errorMessage = 'Passphrase can`t be empty';

            return;
        }

        if (!this.isChecked) {
            this.isError = true;

            return;
        }

        this.onButtonClick();
    }

    /**
     * Changes state to generate passphrase.
     */
    public onChooseGenerate(): void {
        if (this.passphrase && this.isGenerateState) return;

        this.passphrase = bip39.generateMnemonic();
        this.setParentPassphrase(this.passphrase);

        this.isCreateState = false;
        this.isGenerateState = true;
    }

    /**
     * Changes state to create passphrase.
     */
    public onChooseCreate(): void {
        if (this.passphrase && this.isCreateState) return;

        this.errorMessage = '';
        this.passphrase = '';
        this.setParentPassphrase(this.passphrase);

        this.isCreateState = true;
        this.isGenerateState = false;
    }

    /**
     * Holds on copy button click logic.
     * Copies passphrase to clipboard.
     */
    public onCopyClick(): void {
        this.$copyText(this.passphrase);
        this.$notify.success('Passphrase was copied successfully');
    }

    /**
     * Changes passphrase data from input value.
     * @param value
     */
    public onChangePassphrase(value: string): void {
        this.passphrase = value.trim();
        this.setParentPassphrase(this.passphrase);
        this.errorMessage = '';
    }
}
</script>

<style scoped lang="scss">
    .generate-container {
        padding: 25px 50px;
        max-width: 515px;
        min-width: 515px;
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        display: flex;
        flex-direction: column;
        align-items: center;
        background-color: #fff;
        border-radius: 6px;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-weight: bold;
            font-size: 22px;
            line-height: 27px;
            color: #000;
            margin: 0 0 30px 0;
        }

        &__warning {
            display: flex;
            flex-direction: column;
            padding: 20px;
            width: calc(100% - 40px);
            background: #fff9f7;
            border: 1px solid #f84b00;
            margin-bottom: 35px;
            border-radius: 8px;

            &__header {
                display: flex;
                align-items: center;

                &__label {
                    font-style: normal;
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 19px;
                    color: #1b2533;
                    margin: 0 0 0 15px;
                }
            }

            &__message {
                font-size: 16px;
                line-height: 19px;
                color: #1b2533;
                margin: 10px 0 0 0;
            }
        }

        &__choosing {
            display: flex;
            align-items: center;
            justify-content: space-between;
            width: 100%;

            &__label {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
                margin: 0;
            }

            &__right {
                display: flex;
                align-items: center;

                &__option {
                    font-size: 14px;
                    line-height: 17px;
                    color: #768394;
                    margin: 0;
                    cursor: pointer;
                    border-bottom: 3px solid #fff;
                }
            }
        }

        &__value-area {
            margin: 32px 0;
            width: 100%;
            display: flex;
            align-items: flex-start;

            &__mnemonic {
                display: flex;
                background: #f5f6fa;
                border-radius: 9px;
                padding: 10px;
                width: calc(100% - 20px);

                &__value {
                    font-family: 'Source Code Pro', sans-serif;
                    font-size: 14px;
                    line-height: 25px;
                    color: #384b65;
                    word-break: break-word;
                    margin: 0;
                    word-spacing: 8px;
                }

                &__button {
                    margin-left: 10px;
                    min-width: 66px;
                    min-height: 30px;
                }
            }

            &__password {
                width: 100%;

                &__input {
                    width: calc(100% - 8px);
                }
            }
        }

        &__check-area {
            margin-bottom: 32px;
            font-size: 14px;
            line-height: 19px;
            color: #1b2533;

            &__checkbox {
                margin: 0 10px 0 0;
            }
        }
    }

    .left-option {
        margin-right: 15px;
    }

    .active {
        font-family: 'font_bold', sans-serif;
        color: #0068dc;
        border-bottom: 3px solid #0068dc;
    }

    .error {
        color: red;
    }

    /deep/ .label-container {

        &__main {
            margin-bottom: 10px;

            &__label {
                margin: 0;
                font-size: 14px;
                line-height: 19px;
                color: #7c8794;
                font-family: 'font_bold', sans-serif;
            }

            &__error {
                margin: 0 0 0 10px;
                font-size: 14px;
                line-height: 19px;
            }
        }
    }
</style>
