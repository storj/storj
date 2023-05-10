// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="cli-access">
        <p class="cli-access__info">
            Now copy and save the Satellite Address and API Key as they will only appear once.
        </p>
        <ButtonsContainer label="Save your CLI access">
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
        <div class="cli-access__blurred">
            <ValueWithBlur
                class="cli-access__blurred__address"
                button-label="Show Address"
                :is-mnemonic="false"
                :value="satelliteAddress"
                title="Satellite Address"
            />
            <ValueWithBlur
                button-label="Show API Key"
                :is-mnemonic="false"
                :value="apiKey"
                title="API Key"
            />
        </div>
        <ButtonsContainer>
            <template #leftButton>
                <LinkButton
                    label="Learn More"
                    link="https://docs.storj.io/dcs/concepts/access/access-grants/api-key"
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
import { ref, computed } from 'vue';

import { useNotify, useRouter } from '@/utils/hooks';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { Download } from '@/utils/download';
import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { useConfigStore } from '@/store/modules/configStore';

import VButton from '@/components/common/VButton.vue';
import ButtonsContainer from '@/components/accessGrants/createFlow/components/ButtonsContainer.vue';
import ValueWithBlur from '@/components/accessGrants/createFlow/components/ValueWithBlur.vue';
import LinkButton from '@/components/accessGrants/createFlow/components/LinkButton.vue';

const props = defineProps<{
    name: string;
    apiKey: string;
}>();

const configStore = useConfigStore();
const notify = useNotify();
const router = useRouter();

const isCopied = ref<boolean>(false);
const isDownloaded = ref<boolean>(false);

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Returns the web address of this satellite from the store.
 */
const satelliteAddress = computed((): string => {
    return configStore.state.config.satelliteNodeURL;
});

/**
 * Saves CLI access to clipboard.
 */
function onCopy(): void {
    navigator.clipboard.writeText(`${satelliteAddress.value} ${props.apiKey}`);
    isCopied.value = true;
    analytics.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);
    notify.success(`CLI access was copied successfully`);
}

/**
 * Downloads CLI access into .txt file.
 */
function onDownload(): void {
    isDownloaded.value = true;

    const fileContent = `Satellite address:\n${satelliteAddress.value}\n\nAPI Key:\n${props.apiKey}`;
    Download.file(fileContent, `Storj-CLI-access-${props.name}-${new Date().toISOString()}.txt`);
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
.cli-access {
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

        &__address {
            margin-bottom: 16px;
        }
    }
}
</style>
