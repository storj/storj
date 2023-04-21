// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="generated-step">
        <GeneratedIcon />
        <h1 class="generated-step__title">Passphrase Generated</h1>
        <p class="generated-step__info">
            Please note that Storj does not know or store your encryption passphrase. If you lose it, you will not be
            able to recover your files. Save your passphrase and keep it safe.
        </p>
        <div class="generated-step__mnemonic">
            <p class="generated-step__mnemonic__value">
                {{ passphrase }}
            </p>
            <div class="generated-step__mnemonic__buttons">
                <v-button
                    class="copy-button"
                    :label="isPassphraseCopied ? 'Copied' : 'Copy to clipboard'"
                    width="156px"
                    height="40px"
                    :is-white="!isPassphraseCopied"
                    :is-white-green="isPassphraseCopied"
                    font-size="13px"
                    :on-press="onCopyPassphraseClick"
                >
                    <template #icon>
                        <copy-icon v-if="!isPassphraseCopied" class="copy-icon" />
                        <check-icon v-else class="check-icon" />
                    </template>
                </v-button>
                <v-button
                    :label="isPassphraseDownloaded ? 'Downloaded' : 'Download'"
                    font-size="13px"
                    width="100%"
                    height="40px"
                    :is-green="isPassphraseDownloaded"
                    :on-press="downloadPassphrase"
                >
                    <template #icon>
                        <download-icon v-if="!isPassphraseDownloaded" />
                        <check-icon v-else />
                    </template>
                </v-button>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { useCopy, useNotify } from '@/utils/hooks';
import { Download } from '@/utils/download';

import VButton from '@/components/common/VButton.vue';

import GeneratedIcon from '@/../static/images/projectPassphrase/generated.svg';
import CheckIcon from '@/../static/images/common/check.svg';
import CopyIcon from '@/../static/images/common/copy.svg';
import DownloadIcon from '@/../static/images/common/download.svg';

const props = withDefaults(defineProps<{
    passphrase?: string,
}>(), {
    passphrase: '',
});

const notify = useNotify();

const currentDate = new Date().toISOString();

const isPassphraseCopied = ref<boolean>(false);
const isPassphraseDownloaded = ref<boolean>(false);
const analytics = new AnalyticsHttpApi();

/**
 * Copies passphrase to clipboard.
 */
function onCopyPassphraseClick(): void {
    navigator.clipboard.writeText(props.passphrase);
    isPassphraseCopied.value = true;
    analytics.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);
    notify.success(`Passphrase was copied successfully`);
}

/**
 * Downloads passphrase to .txt file.
 */
function downloadPassphrase(): void {
    isPassphraseDownloaded.value = true;
    Download.file(props.passphrase, `passphrase-${currentDate}.txt`);
    analytics.eventTriggered(AnalyticsEvent.DOWNLOAD_TXT_CLICKED);
}
</script>

<style scoped lang="scss">
.generated-step {
    display: flex;
    flex-direction: column;
    align-items: center;
    font-family: 'font_regular', sans-serif;
    max-width: 433px;

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 32px;
        line-height: 39px;
        color: #1b2533;
        margin: 14px 0;
    }

    &__info {
        font-size: 14px;
        line-height: 19px;
        color: #354049;
        margin-bottom: 24px;
    }

    &__mnemonic {
        display: flex;
        align-items: center;
        background: #ebeef1;
        border: 1px solid #d8dee3;
        border-radius: 10px;
        padding: 10px 15px 10px 23px;

        @media screen and (max-width: 530px) {
            flex-direction: column;
        }

        &__value {
            font-size: 16px;
            line-height: 26px;
            letter-spacing: -0.02em;
            color: #091c45;
            text-align: justify;
        }

        &__buttons {
            display: flex;
            flex-direction: column;
            row-gap: 8px;
            margin-left: 14px;

            @media screen and (max-width: 530px) {
                margin: 15px 0 0;
            }
        }
    }
}

.copy-button {
    background-color: #fff !important;
}

.copy-icon {

    :deep(rect),
    :deep(path) {
        stroke: var(--c-grey-6);
    }
}

.check-icon :deep(path) {
    fill: var(--c-green-5);
}
</style>
