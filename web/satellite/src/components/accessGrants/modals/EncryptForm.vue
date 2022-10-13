// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="encrypt">
        <h2 class="encrypt__title">Select Encryption</h2>
        <div
            v-if="!(encryptSelect === 'create' && (isPassphraseDownloaded || isPassphraseCopied))"
            class="encrypt__item"
        >
            <div class="encrypt__item__left-area">
                <AccessKeyIcon
                    class="encrypt__item__left-area__icon"
                    :class="{ selected: encryptSelect === 'generate' }"
                />
                <div class="encrypt__item__left-area__text">
                    <h3>Generate Passphrase</h3>
                    <p>Automatically Generate Seed</p>
                </div>
            </div>
            <div class="encrypt__item__radio">
                <input
                    id="generate-check"
                    v-model="encryptSelect"
                    value="generate"
                    type="radio"
                    name="type"
                    @change="onRadioInput"
                >
            </div>
        </div>
        <div
            v-if="encryptSelect === 'generate'"
            class="encrypt__generated-passphrase"
        >
            {{ passphrase }}
        </div>
        <div
            v-if="!(encryptSelect && (isPassphraseDownloaded || isPassphraseCopied))"
            id="divider"
            class="encrypt__divider"
            :class="{ 'in-middle': encryptSelect === 'generate' }"
        />
        <div
            v-if="!(encryptSelect === 'generate' && (isPassphraseDownloaded || isPassphraseCopied))"
            id="own"
            :class="{ 'in-middle': encryptSelect === 'generate' }"
            class="encrypt__item"
        >
            <div class="encrypt__item__left-area">
                <ThumbPrintIcon
                    class="encrypt__item__left-area__icon"
                    :class="{ selected: encryptSelect === 'create' }"
                />
                <div class="encrypt__item__left-area__text">
                    <h3>Create My Own Passphrase</h3>
                    <p>Make it Personalized</p>
                </div>
            </div>
            <div class="encrypt__item__radio">
                <input
                    id="create-check"
                    v-model="encryptSelect"
                    value="create"
                    type="radio"
                    name="type"
                    @change="onRadioInput"
                >
            </div>
        </div>
        <input
            v-if="encryptSelect === 'create'"
            v-model="passphrase"
            type="text"
            placeholder="Input Your Passphrase"
            class="encrypt__passphrase" :disabled="encryptSelect === 'generate'"
            @input="resetSavedStatus"
        >
        <div
            class="encrypt__footer-container"
            :class="{ 'in-middle': encryptSelect === 'generate' }"
        >
            <div class="encrypt__footer-container__buttons">
                <v-button
                    :label="isPassphraseCopied ? 'Copied' : 'Copy to clipboard'"
                    height="50px"
                    :is-transparent="!isPassphraseCopied"
                    :is-white-green="isPassphraseCopied"
                    class="encrypt__footer-container__buttons__copy-button"
                    font-size="14px"
                    :on-press="onCopyPassphraseClick"
                    :is-disabled="passphrase.length < 1"
                >
                    <template v-if="!isPassphraseCopied" #icon>
                        <copy-icon class="button-icon" :class="{ active: passphrase }" />
                    </template>
                </v-button>
                <v-button
                    label="Download .txt"
                    font-size="14px"
                    height="50px"
                    class="encrypt__footer-container__buttons__download-button"
                    :is-green-white="isPassphraseDownloaded"
                    :on-press="downloadPassphrase"
                    :is-disabled="passphrase.length < 1"
                >
                    <template v-if="!isPassphraseDownloaded" #icon>
                        <download-icon class="button-icon" />
                    </template>
                </v-button>
            </div>
            <div v-if="isPassphraseDownloaded || isPassphraseCopied" :class="`encrypt__footer-container__acknowledgement-container ${acknowledgementCheck ? 'blue-background' : ''}`">
                <input
                    v-model="acknowledgementCheck"
                    type="checkbox"
                    class="encrypt__footer-container__acknowledgement-container__check"
                >
                <div class="encrypt__footer-container__acknowledgement-container__text">I understand that Storj does not know or store my encryption passphrase. If I lose it, I won't be able to recover files.</div>
            </div>
            <div
                v-if="isPassphraseDownloaded || isPassphraseCopied"
                class="encrypt__footer-container__buttons"
            >
                <v-button
                    label="Back"
                    height="50px"
                    :is-transparent="true"
                    class="encrypt__footer-container__buttons__copy-button"
                    font-size="14px"
                    :on-press="backAction"
                />
                <v-button
                    label="Create my Access ⟶"
                    font-size="14px"
                    height="50px"
                    class="encrypt__footer-container__buttons__download-button"
                    :is-disabled="!acknowledgementCheck"
                    :on-press="createAccessGrant"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue, Watch } from 'vue-property-decorator';
import { generateMnemonic } from 'bip39';

import { Download } from '@/utils/download';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import VButton from '@/components/common/VButton.vue';

import CopyIcon from '@/../static/images/common/copy.svg';
import DownloadIcon from '@/../static/images/common/download.svg';
import AccessKeyIcon from '@/../static/images/accessGrants/accessKeyIcon.svg';
import ThumbPrintIcon from '@/../static/images/accessGrants/thumbPrintIcon.svg';

// @vue/component
@Component({
    components: {
        AccessKeyIcon,
        CopyIcon,
        DownloadIcon,
        ThumbPrintIcon,
        VButton,
    },
})

export default class EncryptForm extends Vue {
    private encryptSelect = 'create';
    private isPassphraseCopied = false;
    private isPassphraseDownloaded = false;
    private passphrase = '';
    private acknowledgementCheck = false;
    public currentDate = new Date().toISOString();

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    @Watch('passphrase')
    public applyPassphrase(): void {
        this.$emit('apply-passphrase', this.passphrase);
    }

    public createAccessGrant(): void {
        this.$emit('create-access');
    }

    public onCloseClick(): void {
        this.$emit('close-modal');
    }

    public onRadioInput(): void {
        this.isPassphraseCopied = false;
        this.isPassphraseDownloaded = false;
        this.passphrase = '';

        if (this.encryptSelect === 'generate') {
            this.passphrase = generateMnemonic();
        }
    }

    public backAction(): void {
        this.$emit('backAction');
    }

    public resetSavedStatus(): void {
        this.isPassphraseCopied = false;
        this.isPassphraseDownloaded = false;
    }

    public onCopyPassphraseClick(): void {
        this.$copyText(this.passphrase);
        this.isPassphraseCopied = true;
        this.analytics.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);
        this.$notify.success(`Passphrase was copied successfully`);
    }

    /**
     * Downloads passphrase to .txt file
     */
    public downloadPassphrase(): void {
        this.isPassphraseDownloaded = true;
        Download.file(this.passphrase, `passphrase-${this.currentDate}.txt`);
        this.analytics.eventTriggered(AnalyticsEvent.DOWNLOAD_TXT_CLICKED);
    }
}
</script>

<style scoped lang="scss">
.button-icon {
    margin-right: 5px;

    :deep(path),
    :deep(rect) {
        stroke: white;
    }

    &.active {

        :deep(path),
        :deep(rect) {
            stroke: #56606d;
        }
    }
}

.encrypt {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    justify-content: center;
    font-family: 'font_regular', sans-serif;
    padding: 32px;
    max-width: 350px;

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 28px;
        line-height: 36px;
        letter-spacing: -0.02em;
        color: #000;
        margin-bottom: 32px;
    }

    &__divider {
        width: 100%;
        height: 1px;
        background: #ebeef1;
        margin: 16px 0;

        &.in-middle {
            order: 4;
        }
    }

    &__item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        width: 100%;
        box-sizing: border-box;
        margin-top: 10px;

        &__left-area {
            display: flex;
            align-items: center;
            justify-content: flex-start;

            &__icon {
                margin-right: 8px;

                &.selected {

                    :deep(circle) {
                        fill: #e6edf7 !important;
                    }

                    :deep(path) {
                        fill: #003dc1 !important;
                    }
                }
            }

            &__text {
                display: flex;
                flex-direction: column;
                justify-content: space-between;
                align-items: flex-start;
                font-family: 'font_regular', sans-serif;
                font-size: 12px;

                h3 {
                    margin: 0 0 8px;
                    font-family: 'font_bold', sans-serif;
                    font-size: 14px;
                }

                p {
                    padding: 0;
                }
            }
        }

        &__radio {
            display: flex;
            align-items: center;
            justify-content: center;
            width: 10px;
            height: 10px;
        }
    }

    &__generated-passphrase {
        margin-top: 20px;
        margin-bottom: 20px;
        align-items: center;
        padding: 10px 16px;
        background: #ebeef1;
        border: 1px solid #c8d3de;
        border-radius: 7px;
        text-align: left;
    }

    &__passphrase {
        margin-top: 20px;
        width: 100%;
        background: #fff;
        border: 1px solid #c8d3de;
        box-sizing: border-box;
        border-radius: 4px;
        font-size: 14px;
        padding: 10px;
    }

    &__footer-container {
        display: flex;
        flex-direction: column;
        width: 100%;
        justify-content: flex-start;
        margin-top: 16px;

        &__buttons {
            display: flex;
            width: 100%;
            margin-top: 25px;
            column-gap: 8px;

            @media screen and (max-width: 390px) {
                flex-direction: column;
                column-gap: unset;
                row-gap: 8px;
            }

            &__copy-button,
            &__download-button {
                padding: 0 15px;

                @media screen and (max-width: 390px) {
                    width: unset !important;
                }
            }
        }

        &__acknowledgement-container {
            border: 1px solid #c8d3de;
            border-radius: 6px;
            display: grid;
            grid-template-columns: 1fr 6fr;
            padding: 10px;
            margin-top: 25px;
            height: 80px;
            align-content: center;

            &__check {
                margin: 0 auto auto;
                border-radius: 4px;
                height: 16px;
                width: 16px;
            }

            &__text {
                text-align: left;
            }
        }
    }
}
</style>
