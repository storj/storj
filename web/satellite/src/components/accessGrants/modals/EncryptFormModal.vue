// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="access-grant__modal-container__header-container">
            <h2 class="access-grant__modal-container__header-container__title">Select Encryption</h2>
            <div
                class="access-grant__modal-container__header-container__close-cross-container"
                @click="onCloseClick"
            >
                <CloseCrossIcon />
            </div>
        </div>
        <div class="access-grant__modal-container__body-container-encrypt">
            <div class="access-grant__modal-container__body-container__encrypt">
                <div
                    v-if="!(encryptSelect === 'create' && (isPassphraseDownloaded || isPassphraseCopied))"
                    class="access-grant__modal-container__body-container__encrypt__item"
                >
                    <div class="access-grant__modal-container__body-container__encrypt__item__left-area">
                        <AccessKeyIcon
                            class="access-grant__modal-container__body-container__encrypt__item__icon"
                            :class="{ selected: encryptSelect === 'generate' }"
                        />
                        <div class="access-grant__modal-container__body-container__encrypt__item__text">
                            <h3>Generate Passphrase</h3>
                            <p>Automatically Generate Seed</p>
                        </div>
                    </div>
                    <div class="access-grant__modal-container__body-container__encrypt__item__radio">
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
                    class="access-grant__modal-container__generated-passphrase"
                >
                    {{ passphrase }}
                </div>
                <div
                    v-if="!(encryptSelect && (isPassphraseDownloaded || isPassphraseCopied))"
                    id="divider"
                    class="access-grant__modal-container__body-container__encrypt__divider"
                    :class="{ 'in-middle': encryptSelect === 'generate' }"
                />
                <div
                    v-if="!(encryptSelect === 'generate' && (isPassphraseDownloaded || isPassphraseCopied))"
                    id="own"
                    :class="{ 'in-middle': encryptSelect === 'generate' }"
                    class="access-grant__modal-container__body-container__encrypt__item"
                >
                    <div class="access-grant__modal-container__body-container__encrypt__item__left-area">
                        <ThumbPrintIcon
                            class="access-grant__modal-container__body-container__encrypt__item__icon"
                            :class="{ selected: encryptSelect === 'create' }"
                        />
                        <div class="access-grant__modal-container__body-container__encrypt__item__text">
                            <h3>Create My Own Passphrase</h3>
                            <p>Make it Personalized</p>
                        </div>
                    </div>
                    <div class="access-grant__modal-container__body-container__encrypt__item__radio">
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
                    class="access-grant__modal-container__body-container__passphrase" :disabled="encryptSelect === 'generate'"
                    @input="resetSavedStatus"
                >
                <div
                    class="access-grant__modal-container__footer-container"
                    :class="{ 'in-middle': encryptSelect === 'generate' }"
                >
                    <v-button
                        :label="isPassphraseCopied ? 'Copied' : 'Copy to clipboard'"
                        width="auto"
                        height="50px"
                        :is-transparent="!isPassphraseCopied"
                        :is-white-green="isPassphraseCopied"
                        class="access-grant__modal-container__footer-container__copy-button"
                        font-size="16px"
                        :on-press="onCopyPassphraseClick"
                        :is-disabled="passphrase.length < 1"
                    >
                        <template v-if="!isPassphraseCopied" #icon>
                            <copy-icon class="button-icon" :class="{ active: passphrase }" />
                        </template>
                    </v-button>
                    <v-button
                        label="Download .txt"
                        font-size="16px"
                        width="auto"
                        height="50px"
                        class="access-grant__modal-container__footer-container__download-button"
                        :is-green-white="isPassphraseDownloaded"
                        :on-press="downloadPassphrase"
                        :is-disabled="passphrase.length < 1"
                    >
                        <template v-if="!isPassphraseDownloaded" #icon>
                            <download-icon class="button-icon" />
                        </template>
                    </v-button>
                </div>
            </div>
            <div v-if="isPassphraseDownloaded || isPassphraseCopied" :class="`access-grant__modal-container__acknowledgement-container ${acknowledgementCheck ? 'blue-background' : ''}`">
                <input
                    v-model="acknowledgementCheck"
                    type="checkbox"
                    class="access-grant__modal-container__acknowledgement-container__check"
                >
                <div class="access-grant__modal-container__acknowledgement-container__text">I understand that Storj does not know or store my encryption passphrase. If I lose it, I won't be able to recover files.</div>
            </div>
            <div
                v-if="isPassphraseDownloaded || isPassphraseCopied"
                class="access-grant__modal-container__acknowledgement-buttons"
            >
                <v-button
                    label="Back"
                    width="auto"
                    height="50px"
                    :is-transparent="true"
                    class="access-grant__modal-container__footer-container__copy-button"
                    font-size="16px"
                    :on-press="backAction"
                />
                <v-button
                    label="Create my Access ⟶"
                    font-size="16px"
                    width="auto"
                    height="50px"
                    class="access-grant__modal-container__footer-container__download-button"
                    :is-disabled="!acknowledgementCheck"
                    :on-press="createAccessGrant"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue, Watch } from 'vue-property-decorator';
import { generateMnemonic } from "bip39";
import { Download } from "@/utils/download";

import CopyIcon from '../../../../static/images/common/copy.svg';
import VButton from '@/components/common/VButton.vue';
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import AccessKeyIcon from '@/../static/images/accessGrants/accessKeyIcon.svg';
import ThumbPrintIcon from '@/../static/images/accessGrants/thumbPrintIcon.svg';
import DownloadIcon from '../../../../static/images/common/download.svg';

// @vue/component
@Component({
    components: {
        AccessKeyIcon,
        CloseCrossIcon,
        CopyIcon,
        DownloadIcon,
        ThumbPrintIcon,
        VButton
    },
})

export default class EncryptFormModal extends Vue {

    private encryptSelect = "create";
    private isPassphraseCopied = false;
    private isPassphraseDownloaded = false;
    private passphrase = "";
    private accessGrantStep = "create";
    private acknowledgementCheck = false;
    public currentDate = new Date().toISOString();

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
        this.$emit('backAction')
    }

    public onCopyPassphraseClick(): void {
        this.$copyText(this.passphrase);
        this.isPassphraseCopied = true;
        this.$notify.success(`Passphrase was copied successfully`);
    }

    /**
     * Downloads passphrase to .txt file
     */
    public downloadPassphrase(): void {
        this.isPassphraseDownloaded = true;
        Download.file(this.passphrase, `passphrase-${this.currentDate}.txt`)
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

@mixin generated-text {
    margin-top: 20px;
    margin-bottom: 20px;
    align-items: center;
    padding: 10px 16px;
    background: #ebeef1;
    border: 1px solid #c8d3de;
    border-radius: 7px;
}

.access-grant {
    position: fixed;
    top: 0;
    bottom: 0;
    left: 0;
    right: 0;
    z-index: 100;
    background: rgb(27 37 51 / 75%);
    display: flex;
    align-items: flex-start;
    justify-content: center;

    & > * {
        font-family: sans-serif;
    }

    &__modal-container {
        background: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        position: relative;
        padding: 25px 40px;
        margin-top: 40px;
        width: 410px;
        height: auto;

        &__generated-passphrase {
            @include generated-text;
        }

        &__generated-credentials {
            @include generated-text;

            margin: 0 0 4px;
            display: flex;
            justify-content: space-between;

            &__text {
                width: 90%;
                text-overflow: ellipsis;
                overflow-x: hidden;
                white-space: nowrap;
            }
        }

        &__header-container {
            text-align: left;
            display: grid;
            grid-template-columns: 2fr 1fr;
            width: 100%;
            padding-top: 10px;

            &__title {
                grid-column: 1;
            }

            &__close-cross-container {
                grid-column: 2;
                margin: auto 0 auto auto;
                display: flex;
                justify-content: center;
                align-items: center;
                right: 30px;
                top: 30px;
                height: 24px;
                width: 24px;
                cursor: pointer;
            }

            &__close-cross-container:hover .close-cross-svg-path {
                fill: #2683ff;
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
                font-family: sans-serif;
            }
        }

        &__acknowledgement-buttons {
            display: flex;
            padding-top: 25px;
        }

        &__body-container {
            display: grid;
            grid-template-columns: 1fr 6fr;
            grid-template-rows: auto auto auto auto auto auto;
            grid-row-gap: 24px;
            width: 100%;
            padding-top: 10px;
            margin-top: 24px;

            &__passphrase {
                margin-top: 20px;
                width: 100%;
                background: #fff;
                border: 1px solid #c8d3de;
                box-sizing: border-box;
                border-radius: 4px;
                height: 40px;
                font-size: 17px;
                padding: 10px;
            }

            &__encrypt {
                width: 100%;
                display: flex;
                flex-flow: column;
                align-items: center;
                justify-content: center;
                margin: 15px 0;

                &__item {
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    width: 100%;
                    height: 40px;
                    box-sizing: border-box;

                    &__left-area {
                        display: flex;
                        align-items: center;
                        justify-content: flex-start;
                    }

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

                    &__radio {
                        display: flex;
                        align-items: center;
                        justify-content: center;
                        width: 10px;
                        height: 10px;
                    }
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
            }
        }

        &__footer-container {
            display: flex;
            width: 100%;
            justify-content: flex-start;
            margin-top: 16px;

            & :deep(.container:first-of-type) {
                margin-right: 8px;
            }

            &__copy-button {
                width: 49% !important;
                margin-right: 10px;
            }

            &__download-button {
                width: 49% !important;
            }

            .in-middle {
                order: 3;
            }
        }
    }
}
</style>
