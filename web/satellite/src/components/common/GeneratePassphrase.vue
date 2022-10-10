// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="encrypt-container">
        <EncryptIcon />
        <h1 class="encrypt-container__title" aria-roledescription="enc-title">Encryption passphrase</h1>
        <p class="encrypt-container__info">
            The encryption passphrase is used to encrypt and access the data that you upload to Storj.
        </p>
        <div class="encrypt-container__functional">
            <div class="encrypt-container__functional__header">
                <p class="encrypt-container__functional__header__gen" :class="{ active: isGenerate }" @click="setToGenerate">
                    Generate a new passphrase
                </p>
                <div class="encrypt-container__functional__header__right" :class="{ active: !isGenerate }">
                    <p
                        class="encrypt-container__functional__header__right__enter"
                        :class="{ active: !isGenerate }"
                        aria-roledescription="enter-passphrase-label"
                        @click="setToEnter"
                    >
                        Enter your own passphrase
                    </p>
                    <VInfo class="encrypt-container__functional__header__right__info-button">
                        <template #icon>
                            <InfoIcon class="encrypt-container__functional__header__right__info-button__image" :class="{ active: !isGenerate }" />
                        </template>
                        <template #message>
                            <p class="encrypt-container__functional__header__right__info-button__message">
                                We strongly encourage you to use a mnemonic phrase, which is automatically generated on
                                the client-side for you. Alternatively, you can enter your own passphrase.
                            </p>
                        </template>
                    </VInfo>
                </div>
            </div>
            <div v-if="isGenerate" class="encrypt-container__functional__generate">
                <p class="encrypt-container__functional__generate__value">{{ passphrase }}</p>
                <VButton
                    class="encrypt-container__functional__generate__button"
                    label="Copy"
                    width="66px"
                    height="30px"
                    is-blue-white="true"
                    is-uppercase="true"
                    :on-press="onCopyClick"
                />
            </div>
            <div v-else class="encrypt-container__functional__enter">
                <VInput
                    label="Your Passphrase"
                    placeholder="Enter a passphrase here..."
                    :error="enterError"
                    role-description="passphrase"
                    is-password="true"
                    :disabled="isLoading"
                    @setData="setPassphrase"
                />
            </div>
            <h2 class="encrypt-container__functional__warning-title" aria-roledescription="warning-title">
                Save your encryption passphrase
            </h2>
            <p class="encrypt-container__functional__warning-msg">
                Please note that Storj does not know or store your encryption passphrase. If you lose it, you will not
                be able to recover your files.
            </p>
            <p class="encrypt-container__functional__download" @click="onDownloadClick">Download as a text file</p>
            <VCheckbox
                class="encrypt-container__functional__checkbox"
                label="I understand, and I have saved the passphrase."
                :is-checkbox-error="isCheckboxError"
                @setData="setSavingConfirmation"
            />
        </div>
        <div class="encrypt-container__buttons">
            <VButton
                v-if="isNewObjectsFlow"
                class="encrypt-container__buttons__back"
                label="< Back"
                height="64px"
                border-radius="62px"
                is-blue-white="true"
                :on-press="onBackClick"
                :is-disabled="isLoading"
            />
            <VButton
                label="Next >"
                height="64px"
                border-radius="62px"
                :on-press="onNextButtonClick"
                :is-disabled="isLoading || !isSavingConfirmed"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';
import { generateMnemonic } from 'bip39';

import { LocalData, UserIDPassSalt } from '@/utils/localData';
import { Download } from '@/utils/download';

import VButton from '@/components/common/VButton.vue';
import VInfo from '@/components/common/VInfo.vue';
import VInput from '@/components/common/VInput.vue';
import VCheckbox from '@/components/common/VCheckbox.vue';

import EncryptIcon from '@/../static/images/objects/encrypt.svg';
import InfoIcon from '@/../static/images/common/smallGreyInfo.svg';

// @vue/component
@Component({
    components: {
        EncryptIcon,
        InfoIcon,
        VInfo,
        VButton,
        VInput,
        VCheckbox,
    },
})
export default class GeneratePassphrase extends Vue {
    @Prop({ default: () => () => null })
    public readonly onNextClick: () => unknown;
    @Prop({ default: () => () => null })
    public readonly onBackClick: () => unknown;
    @Prop({ default: () => () => null })
    public readonly setParentPassphrase: (passphrase: string) => void;
    @Prop({ default: false })
    public readonly isLoading: boolean;

    public isGenerate = true;
    public enterError = '';
    public passphrase = '';
    public isSavingConfirmed = false;
    public isCheckboxError = false;

    /**
     * Lifecycle hook after initial render.
     * Chooses correct state and generates mnemonic.
     */
    public mounted(): void {
        const idPassSalt: UserIDPassSalt | null = LocalData.getUserIDPassSalt();
        if (idPassSalt && idPassSalt.userId === this.$store.getters.user.id) {
            this.isGenerate = false;

            return;
        }

        this.passphrase = generateMnemonic();
        this.setParentPassphrase(this.passphrase);
    }

    public setSavingConfirmation(value: boolean): void {
        this.isSavingConfirmed = value;
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
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        if (this.enterError) this.enterError = '';

        this.passphrase = passphrase;
        this.setParentPassphrase(this.passphrase);
    }

    /**
     * Sets view state to enter passphrase.
     */
    public setToEnter(): void {
        this.passphrase = '';
        this.setParentPassphrase(this.passphrase);
        this.isGenerate = false;
    }

    /**
     * Sets view state to generate passphrase.
     */
    public setToGenerate(): void {
        if (this.enterError) this.enterError = '';

        this.passphrase = generateMnemonic();
        this.setParentPassphrase(this.passphrase);
        this.isGenerate = true;
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

    /**
     * Returns objects flow status from store.
     */
    public get isNewObjectsFlow(): string {
        return this.$store.state.appStateModule.isNewObjectsFlow;
    }
}
</script>

<style scoped lang="scss">
    .encrypt-container {
        font-family: 'font_regular', sans-serif;
        padding: 40px 60px 60px;
        max-width: 500px;
        background: #fcfcfc;
        box-shadow: 0 0 32px rgb(0 0 0 / 4%);
        border-radius: 20px;
        margin: 30px auto 0;
        display: flex;
        flex-direction: column;
        align-items: center;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 36px;
            line-height: 56px;
            letter-spacing: 1px;
            color: #14142b;
            margin: 10px 0;
        }

        &__info {
            font-size: 16px;
            line-height: 28px;
            letter-spacing: 0.75px;
            color: #1b2533;
            margin-bottom: 20px;
            text-align: center;
            max-width: 420px;
        }

        &__functional {
            border: 1px solid #e6e9ef;
            border-radius: 10px;
            padding: 20px 0;

            &__header {
                width: calc(100% - 50px);
                display: flex;
                align-items: center;
                justify-content: space-between;
                padding: 0 25px;
                border-bottom: 1px solid #e6e9ef;

                &__gen {
                    font-family: 'font_medium', sans-serif;
                    font-size: 14px;
                    line-height: 17px;
                    color: #a9b5c1;
                    padding-bottom: 20px;
                    border-bottom: 4px solid #fff;
                    cursor: pointer;
                    white-space: nowrap;
                }

                &__right {
                    display: flex;
                    align-items: flex-start;
                    padding-bottom: 20px;
                    border-bottom: 4px solid #fff;
                    cursor: pointer;

                    &__enter {
                        font-family: 'font_medium', sans-serif;
                        font-size: 14px;
                        line-height: 17px;
                        color: #a9b5c1;
                        margin-right: 10px;
                        white-space: nowrap;
                    }

                    &__info-button {

                        &__image {
                            cursor: pointer;
                        }

                        &__message {
                            color: #586c86;
                            font-size: 16px;
                            line-height: 21px;
                        }
                    }
                }
            }

            &__generate {
                display: flex;
                align-items: center;
                padding: 16px 22px;
                background: #eff0f7;
                border-radius: 10px;
                margin: 25px 25px 0;

                &__value {
                    font-size: 14px;
                    line-height: 25px;
                    color: #384b65;
                }

                &__button {
                    margin-left: 32px;
                    min-width: 66px;
                }
            }

            &__enter {
                margin: 25px 25px 0;
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
                margin: 0 25px;
            }
        }

        &__buttons {
            width: 100%;
            display: flex;
            align-items: center;
            margin-top: 30px;

            &__back {
                margin-right: 24px;
            }
        }
    }

    .active {
        color: #0149ff;
        border-color: #0149ff;
    }

    .active svg rect {
        fill: #0149ff;
    }

    :deep(.info__box__message) {
        min-width: 440px;
    }
</style>
