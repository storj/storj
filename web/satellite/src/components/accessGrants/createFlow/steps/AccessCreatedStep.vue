// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="access-created">
        <p class="access-created__info">
            Now copy or download to save the Access Grant as it will only appear once.
        </p>
        <ButtonsContainer label="Save your access grant">
            <template #leftButton>
                <VButton
                    :label="isCopied ? 'Copied' : 'Copy'"
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
                    :label="isDownloaded ? 'Downloaded' : 'Download'"
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
        <div class="access-created__blurred">
            <ValueWithBlur
                button-label="Show Access Grant"
                :is-mnemonic="false"
                :value="accessGrant"
                title="Access Grant"
            />
        </div>
        <p v-if="hasNextStep" class="access-created__disclaimer">To view your S3 credentials, click next.</p>
        <ButtonsContainer>
            <template #leftButton>
                <LinkButton
                    label="Learn More"
                    link="https://docs.storj.io/dcs/concepts/access/access-grants"
                />
            </template>
            <template #rightButton>
                <VButton
                    :label="hasNextStep ? 'Next' : 'Finish'"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="onNextButtonClick"
                />
            </template>
        </ButtonsContainer>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';

import { useNotify } from '@/utils/hooks';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { Download } from '@/utils/download';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AccessType } from '@/types/createAccessGrant';
import { RouteConfig } from '@/types/router';

import VButton from '@/components/common/VButton.vue';
import ButtonsContainer from '@/components/accessGrants/createFlow/components/ButtonsContainer.vue';
import ValueWithBlur from '@/components/accessGrants/createFlow/components/ValueWithBlur.vue';
import LinkButton from '@/components/accessGrants/createFlow/components/LinkButton.vue';

const props = defineProps<{
    accessTypes: AccessType[];
    name: string;
    accessGrant: string;
    onContinue: () => void;
}>();

const notify = useNotify();
const router = useRouter();

const isCopied = ref<boolean>(false);
const isDownloaded = ref<boolean>(false);

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Indicates if there are S3 credentials to show.
 */
const hasNextStep = computed((): boolean => {
    return props.accessTypes.includes(AccessType.S3) && props.accessTypes.includes(AccessType.AccessGrant);
});

/**
 * Saves passphrase to clipboard.
 */
function onCopy(): void {
    navigator.clipboard.writeText(props.accessGrant);
    isCopied.value = true;
    analytics.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);
    notify.success(`Access Grant was copied successfully`);
}

/**
 * Downloads passphrase into .txt file.
 */
function onDownload(): void {
    isDownloaded.value = true;
    Download.file(props.accessGrant, `Storj-access-${props.name}-${new Date().toISOString()}.txt`);
    analytics.eventTriggered(AnalyticsEvent.DOWNLOAD_TXT_CLICKED);
}

/**
 * Holds on Next/Finish button click logic.
 */
function onNextButtonClick(): void {
    if (hasNextStep.value) {
        props.onContinue();
        return;
    }

    router.push(RouteConfig.AccessGrants.path);
}
</script>

<style lang="scss" scoped>
.access-created {
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
    }

    &__disclaimer {
        font-size: 14px;
        line-height: 20px;
        color: #1b2533;
        margin-top: 16px;
        text-align: left;
    }
}
</style>
