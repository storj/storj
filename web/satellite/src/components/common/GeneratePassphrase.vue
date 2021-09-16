// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="encrypt-container">
        <EncryptIcon />
        <h1 class="encrypt-container__title">Encrypt your data</h1>
        <p class="encrypt-container__info">
            The encryption passphrase is used to encrypt and access the data that you upload to Storj DCS. We strongly
            encourage you to use a mnemonic phrase, which is automatically generated one on the client-side for you.
        </p>
        <div class="encrypt-container__header">
            <p class="encrypt-container__header__rec">RECOMMENDED</p>
            <div class="encrypt-container__header__row">
                <p class="encrypt-container__header__row__gen" :class="{ active: isGenerate }" @click="setToGenerate">Generate Phrase</p>
                <div class="encrypt-container__header__row__right">
                    <p class="encrypt-container__header__row__right__enter" :class="{ active: !isGenerate }" aria-roledescription="enter-passphrase-label" @click="setToEnter">
                        Enter Your Own Passphrase
                    </p>
                    <VInfo class="encrypt-container__header__row__right__info-button">
                        <template #icon>
                            <InfoIcon class="encrypt-container__header__row__right__info-button__image" />
                        </template>
                        <template #message>
                            <p class="encrypt-container__header__row__right__info-button__message">
                                We strongly encourage you to use a mnemonic phrase, which is automatically generated one
                                on the client-side for you. Alternatively, you can enter your own passphrase.
                            </p>
                        </template>
                    </VInfo>
                </div>
            </div>
        </div>
        <div v-if="isGenerate" class="encrypt-container__generate">
            <p class="encrypt-container__generate__value">{{ passphrase }}</p>
            <VButton
                class="encrypt-container__generate__button"
                label="Copy"
                width="66px"
                height="30px"
                is-blue-white="true"
                :on-press="onCopyClick"
            />
        </div>
        <div v-else class="encrypt-container__enter">
            <HeaderlessInput
                placeholder="Enter a passphrase here..."
                :error="enterError"
                role-description="passphrase"
                @setData="setPassphrase"
            />
        </div>
        <p class="encrypt-container__download" @click="onDownloadClick">Download as a text file</p>
        <div class="encrypt-container__warning">
            <h2 class="encrypt-container__warning__title" aria-roledescription="warning-title">The object browser uses server side encryption.</h2>
            <p class="encrypt-container__warning__msg">
                If you want to use our product with only end-to-end encryption, you may want to use our command line solution.
            </p>
        </div>
        <div class="encrypt-container__buttons">
            <VButton
                label="Next >"
                height="64px"
                border-radius="62px"
                :on-press="onNextButtonClick"
                :is-disabled="isLoading"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';
import * as bip39 from "bip39";

import { LocalData, UserIDPassSalt } from "@/utils/localData";
import { Download } from "@/utils/download";

import VButton from '@/components/common/VButton.vue';
import VInfo from "@/components/common/VInfo.vue";
import HeaderlessInput from "@/components/common/HeaderlessInput.vue";

import EncryptIcon from "@/../static/images/objects/encrypt.svg";
import InfoIcon from "@/../static/images/common/greyInfo.svg";

// @vue/component
@Component({
    components: {
        EncryptIcon,
        InfoIcon,
        VInfo,
        VButton,
        HeaderlessInput,
    },
})
export default class GeneratePassphrase extends Vue {
    @Prop({ default: () => null })
    public readonly onNextClick: () => unknown;
    @Prop({ default: () => null })
    public readonly setParentPassphrase: (passphrase: string) => void;
    @Prop({ default: false })
    public readonly isLoading: boolean;

    public isGenerate = true;
    public enterError = '';
    public passphrase = '';

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

        this.passphrase = bip39.generateMnemonic();
        this.setParentPassphrase(this.passphrase);
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
        this.isGenerate = false;
    }

    /**
     * Sets view state to generate passphrase.
     */
    public setToGenerate(): void {
        if (this.enterError) this.enterError = '';

        this.passphrase = bip39.generateMnemonic();
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

        await this.onNextClick();
    }
}
</script>

<style scoped lang="scss">
    .encrypt-container {
        font-family: 'font_regular', sans-serif;
        padding: 60px;
        max-width: 500px;
        background: #fcfcfc;
        box-shadow: 0 0 32px rgba(0, 0, 0, 0.04);
        border-radius: 20px;
        margin: 30px auto 0 auto;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 36px;
            line-height: 56px;
            letter-spacing: 1px;
            color: #14142b;
            margin: 35px 0 10px 0;
        }

        &__info {
            font-size: 16px;
            line-height: 32px;
            letter-spacing: 0.75px;
            color: #1b2533;
            margin-bottom: 20px;
        }

        &__header {

            &__rec {
                font-size: 12px;
                line-height: 15px;
                color: #1b2533;
                opacity: 0.4;
                margin-bottom: 15px;
            }

            &__row {
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__gen {
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 19px;
                    color: #a9b5c1;
                    padding-bottom: 10px;
                    border-bottom: 5px solid #fff;
                    cursor: pointer;
                }

                &__right {
                    display: flex;
                    align-items: flex-start;

                    &__enter {
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                        line-height: 19px;
                        color: #a9b5c1;
                        cursor: pointer;
                        margin-right: 10px;
                        padding-bottom: 10px;
                        border-bottom: 5px solid #fff;
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
        }

        &__generate {
            margin-top: 25px;
            display: flex;
            align-items: center;
            padding: 25px;
            background: #eff0f7;
            border-radius: 10px;

            &__value {
                font-size: 16px;
                line-height: 28px;
                color: #384b65;
            }

            &__button {
                margin-left: 32px;
                min-width: 66px;
            }
        }

        &__enter {
            margin-top: 25px;
        }

        &__download {
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 19px;
            color: #0068dc;
            cursor: pointer;
            margin: 20px 0;
        }

        &__warning {
            border: 1px solid #e6e9ef;
            border-radius: 10px;
            padding: 25px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 19px;
                color: #df1616;
                margin-bottom: 10px;
            }

            &__msg {
                font-size: 14px;
                line-height: 20px;
                color: #1b2533;
                margin-bottom: 10px;
            }
        }

        &__buttons {
            width: 100%;
            display: flex;
            align-items: center;
            margin-top: 30px;

            &__back,
            &__skip {
                margin-right: 24px;
            }
        }
    }

    .active {
        color: #0149ff;
        border-color: #0149ff;
    }

    ::v-deep .info__box__message {
        min-width: 440px;
    }
</style>
