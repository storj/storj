// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="encrypt-container">
        <bucket-icon class="bucket-icon" />
        <div v-if="generationStep === GenerationSteps.TypeSelection" class="encrypt-container__functional">
            <div class="encrypt-container__functional__header">
                <p class="encrypt-container__functional__header__title" aria-roledescription="title">
                    Encrypt your bucket
                </p>
                <p class="encrypt-container__functional__header__info">
                    We encourage you to generate the encryption passphrase.
                    You can also enter your own passphrase for this bucket.
                </p>
            </div>
            <div class="encrypt-container__functional__variant" aria-roledescription="generate" @click="selectedType = GenerationSteps.Generation">
                <input
                    class="encrypt-container__functional__variant__radio"
                    type="radio"
                    name="radio"
                    :checked="selectedType === GenerationSteps.Generation"
                >
                <div class="encrypt-container__functional__variant__icon">
                    <key-icon />
                </div>
                <div class="encrypt-container__functional__variant__text-container">
                    <h4 class="encrypt-container__functional__variant__text-container__title">Generate passphrase</h4>
                    <p class="encrypt-container__functional__variant__text-container__info">Automatically generate 12-word passphrase.</p>
                </div>
            </div>
            <div class="encrypt-container__functional__variant__divider" />
            <div class="encrypt-container__functional__variant" aria-roledescription="manual" @click="selectedType = GenerationSteps.Manual">
                <input
                    class="encrypt-container__functional__variant__radio"
                    type="radio"
                    name="radio"
                    :checked="selectedType === GenerationSteps.Manual"
                >
                <div class="encrypt-container__functional__variant__icon">
                    <fingerprint-icon />
                </div>
                <div class="encrypt-container__functional__variant__text-container">
                    <h4 class="encrypt-container__functional__variant__text-container__title">Enter passphrase</h4>
                    <p class="encrypt-container__functional__variant__text-container__info">You can also enter your own passphrase.</p>
                </div>
            </div>
        </div>
        <div v-else class="encrypt-container__functional">
            <div class="encrypt-container__functional__header">
                <p class="encrypt-container__functional__header__title" aria-roledescription="title">
                    {{ generationStep === GenerationSteps.Generation ? 'Generate a passphrase' : 'Enter a passphrase' }}
                </p>
                <p class="encrypt-container__functional__header__info">
                    Please note that Storj does not know or store your encryption passphrase.
                    If you lose it, you will not be able to recover your files.
                </p>
            </div>
            <div v-if="generationStep === GenerationSteps.Generation" class="encrypt-container__functional__generate">
                <p class="encrypt-container__functional__generate__value" aria-roledescription="mnemonic">{{ passphrase }}</p>
                <v-button
                    class="encrypt-container__functional__generate__button"
                    label="Download"
                    width="143px"
                    height="48px"
                    :on-press="onDownloadClick"
                />
            </div>
            <div v-else class="encrypt-container__functional__enter">
                <v-input
                    label="Your Passphrase"
                    placeholder="Enter a passphrase here..."
                    :error="enterError"
                    role-description="passphrase"
                    is-password="true"
                    :disabled="isLoading"
                    @setData="setPassphrase"
                />
            </div>
            <v-checkbox
                class="encrypt-container__functional__checkbox"
                label="I understand, and I have saved the passphrase."
                :is-checkbox-error="isCheckboxError"
                @setData="setSavingConfirmation"
            />
        </div>
        <div class="encrypt-container__buttons">
            <v-button
                class="encrypt-container__buttons__back button"
                label="Back"
                height="48px"
                width="45%"
                :is-white="true"
                :on-press="generationStep === GenerationSteps.TypeSelection ? onBackClick : navigateToTypeSelection"
                :is-disabled="isLoading"
            />
            <v-button
                v-if="generationStep === GenerationSteps.TypeSelection"
                class="button"
                height="48px"
                width="45%"
                label="Continue"
                :on-press="selectPassphraseVariant"
            />
            <v-button
                v-else
                class="button"
                label="Continue"
                height="48px"
                width="45%"
                :on-press="onNextButtonClick"
                :is-disabled="isLoading || !isSavingConfirmed"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';
import { generateMnemonic } from 'bip39';

import { Download } from '@/utils/download';

import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';
import VCheckbox from '@/components/common/VCheckbox.vue';

import BucketIcon from '@/../static/images/objects/bucketCreation.svg';
import KeyIcon from '@/../static/images/objects/key.svg';
import FingerprintIcon from '@/../static/images/objects/fingerprint.svg';

enum GenerationSteps {
    TypeSelection,
    Generation,
    Manual,
}

// @vue/component
@Component({
    components: {
        BucketIcon,
        KeyIcon,
        FingerprintIcon,
        VButton,
        VInput,
        VCheckbox,
    },
})
export default class GeneratePassphrase extends Vue {
    @Prop({ default: () => null })
    public readonly onNextClick: () => unknown;
    @Prop({ default: () => null })
    public readonly onBackClick: () => unknown;
    @Prop({ default: () => null })
    public readonly setParentPassphrase: (passphrase: string) => void;
    @Prop({ default: false })
    public readonly isLoading: boolean;

    public readonly GenerationSteps = GenerationSteps;

    public selectedType: GenerationSteps = GenerationSteps.Generation;
    public generationStep: GenerationSteps = GenerationSteps.TypeSelection;
    public enterError = '';
    public passphrase = '';
    public isSavingConfirmed = false;
    public isCheckboxError = false;

    public setSavingConfirmation(value: boolean): void {
        this.isSavingConfirmed = value;
    }

    /**
     * Selects passphrase setup variant.
     * If not manual, generates passphrase.
     */
    public selectPassphraseVariant(): void {
        if (this.selectedType === GenerationSteps.Generation) {
            this.passphrase = generateMnemonic();
            this.setParentPassphrase(this.passphrase);
        }

        this.generationStep = this.selectedType;
    }

    /**
     * Holds on download button click logic.
     * Downloads encryption passphrase as a txt file.
     */
    public onDownloadClick(): void {
        if (!this.passphrase) {
            this.enterError = 'Can\'t be empty!';

            return;
        }

        const fileName = 'StorjEncryptionPassphrase.txt';

        Download.file(this.passphrase, fileName);
    }

    /**
     * Navigates back to passphrase option selection.
     */
    public navigateToTypeSelection(): void {
        this.enterError = '';
        this.passphrase = '';
        this.isSavingConfirmed = false;
        this.generationStep = GenerationSteps.TypeSelection;
    }

    /**
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        if (this.enterError) this.enterError = '';

        this.passphrase = passphrase;
        this.setParentPassphrase(this.passphrase);
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextButtonClick(): Promise<void> {
        if (!this.passphrase) {
            this.enterError = 'Can\'t be empty!';

            return;
        }

        if (!this.isSavingConfirmed) {
            this.isCheckboxError = true;

            return;
        }

        await this.onNextClick();
    }
}
</script>

<style lang="scss">
.encrypt-container {
    font-family: 'font_regular', sans-serif;
    padding: 60px 60px 50px;
    max-width: 500px;
    background: #fcfcfc;
    box-shadow: 0 0 32px rgb(0 0 0 / 4%);
    border-radius: 20px;
    margin: 30px auto 0;
    display: flex;
    flex-direction: column;
    align-items: center;

    &__functional {
        margin-top: 20px;

        &__header {
            display: flex;
            align-items: center;
            justify-content: center;
            flex-direction: column;
            padding: 25px 0;
            text-align: center;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 26px;
                line-height: 31px;
                color: #131621;
                margin-bottom: 15px;
            }

            &__info {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
            }
        }

        &__variant {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            padding: 20px 0;
            cursor: pointer;

            &__radio {
                color: #0149ff;
            }

            &__icon {
                display: flex;
                align-items: center;
                justify-content: center;
                width: 40px;
                height: 40px;
                border-radius: 50%;
                background: #e6edf7;
                margin: 0 8px;

                svg {
                    width: 16px;
                    height: 16px;
                    max-width: 16px;
                    max-height: 16px;
                }
            }

            &__text-container {
                display: flex;
                flex-direction: column;
                justify-content: space-between;
                align-items: flex-start;
                width: 100%;

                &__title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 14px;
                    line-height: 20px;
                    color: #000;
                }

                &__info {
                    font-family: 'font_regular', sans-serif;
                    font-size: 12px;
                    line-height: 18px;
                    color: #000;
                }
            }

            &__divider {
                height: 1px;
                width: 100%;
                background: #c8d3de;
            }
        }

        &__generate {
            display: flex;
            align-items: center;
            padding: 16px 22px;
            background: #eff0f7;
            border-radius: 10px;
            margin-top: 25px;

            &__value {
                font-size: 14px;
                line-height: 25px;
                color: #384b65;
            }

            &__button {
                margin-left: 12px;
                min-width: 130px;
            }
        }

        &__download {
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 19px;
            color: #0068dc;
            cursor: pointer;
            margin: 20px 25px;
            display: inline-block;
        }

        &__warning-title {
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 19px;
            color: #1b2533;
            margin: 25px 25px 10px;
        }

        &__warning-msg {
            font-size: 14px;
            line-height: 20px;
            color: #1b2533;
            margin: 0 25px;
        }

        &__checkbox {
            margin-top: 25px;
        }
    }

    &__buttons {
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-top: 30px;

        &__back {
            margin-right: 24px;
        }
    }
}

@media screen and (max-width: 760px) {

    .encrypt-container {
        padding: 40px 32px;
    }
}

@media screen and (max-width: 600px) {

    .encrypt-container {

        &__functional__header {

            &__title {
                font-size: 1.715rem;
                line-height: 2.215rem;
            }

            &__info {
                font-size: 0.875rem;
                line-height: 1.285rem;
            }
        }

        &__buttons {
            flex-direction: column-reverse;
            margin-top: 25px;

            .button {
                width: 100% !important;

                &:first-of-type {
                    margin: 25px 0 0;
                }
            }
        }
    }
}

@media screen and (max-width: 385px) {

    .encrypt-container {
        padding: 20px;
    }
}
</style>
