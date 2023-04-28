// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="credentials">
        <p class="credentials__info">
            Now copy and save the S3 credentials as they will only appear once.
        </p>
        <ButtonsContainer label="Save your S3 credentials">
            <template #leftButton>
                <VButton
                    :label="isCopied ? 'Copied' : 'Copy all'"
                    width="100%"
                    height="40px"
                    font-size="14px"
                    :on-press="onCopy"
                    :icon="isCopied ? 'check' : 'copy'"
                    :is-white="!isCopied"
                    :is-white-green="isCopied"
                />
            </template>
            <template #rightButton>
                <VButton
                    :label="isDownloaded ? 'Downloaded' : 'Download all'"
                    width="100%"
                    height="40px"
                    font-size="14px"
                    :on-press="onDownload"
                    :icon="isDownloaded ? 'check' : 'download'"
                    :is-white="!isDownloaded"
                    :is-white-green="isDownloaded"
                />
            </template>
        </ButtonsContainer>
        <div class="credentials__blurred">
            <ValueWithBlur
                class="credentials__blurred__with-margin"
                button-label="Show Access Key"
                :is-mnemonic="false"
                :value="credentials.accessKeyId"
                title="Access Key"
            />
            <ValueWithBlur
                class="credentials__blurred__with-margin"
                button-label="Show Secret Key"
                :is-mnemonic="false"
                :value="credentials.secretKey"
                title="Secret Key"
            />
            <ValueWithBlur
                button-label="Show Endpoint"
                :is-mnemonic="false"
                :value="credentials.endpoint"
                title="Endpoint"
            />
        </div>
        <ButtonsContainer>
            <template #leftButton>
                <LinkButton
                    label="View Docs"
                    link="https://docs.storj.io/dcs/api-reference/s3-compatible-gateway"
                />
            </template>
            <template #rightButton>
                <VButton
                    label="Finish"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="onFinishButtonClick"
                />
            </template>
        </ButtonsContainer>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useNotify, useRouter } from '@/utils/hooks';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { Download } from '@/utils/download';
import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { EdgeCredentials } from '@/types/accessGrants';

import VButton from '@/components/common/VButton.vue';
import ButtonsContainer from '@/components/accessGrants/createFlow/components/ButtonsContainer.vue';
import ValueWithBlur from '@/components/accessGrants/createFlow/components/ValueWithBlur.vue';
import LinkButton from '@/components/accessGrants/createFlow/components/LinkButton.vue';

const props = defineProps<{
    name: string;
    credentials: EdgeCredentials;
}>();

const notify = useNotify();
const router = useRouter();

const isCopied = ref<boolean>(false);
const isDownloaded = ref<boolean>(false);

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Saves CLI access to clipboard.
 */
function onCopy(): void {
    const { credentials } = props;
    navigator.clipboard.writeText(`${credentials.accessKeyId} ${credentials.secretKey} ${credentials.endpoint}`);

    isCopied.value = true;
    analytics.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);
    notify.success(`S3 credentials were copied successfully`);
}

/**
 * Downloads CLI access into .txt file.
 */
function onDownload(): void {
    isDownloaded.value = true;

    const fileContent = `Access Key:\n${props.credentials.accessKeyId}\n\nSecret Key:\n${props.credentials.secretKey}\n\nEndpoint:\n${props.credentials.endpoint}`;
    Download.file(fileContent, `Storj-S3-credentials-${props.name}-${new Date().toISOString()}.txt`);
    analytics.eventTriggered(AnalyticsEvent.DOWNLOAD_TXT_CLICKED);
}

/**
 * Holds on Finish button click logic.
 */
function onFinishButtonClick(): void {
    router.push(RouteConfig.AccessGrants.path);
}
</script>

<style lang="scss" scoped>
.credentials {
    font-family: 'font_regular', sans-serif;

    &__info {
        font-size: 14px;
        line-height: 20px;
        color: #091c45;
        padding: 16px 0;
        margin-bottom: 16px;
        border-bottom: 1px solid #ebeef1;
        text-align: left;
    }

    &__blurred {
        margin-top: 16px;
        padding: 16px 0;
        border-top: 1px solid #ebeef1;
        border-bottom: 1px solid #ebeef1;

        &__with-margin {
            margin-bottom: 16px;
        }
    }
}
</style>
