// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="warning-step">
        <p class="warning-step__info">
            By generating S3 credentials, you are opting in to
            <a
                class="warning-step__info__link"
                href="https://docs.storj.io/dcs/concepts/encryption-key/design-decision-server-side-encryption/"
                target="_blank"
                rel="noopener noreferrer"
                @click="trackPageVisit"
            >server-side encryption</a>.
        </p>
        <Toggle
            :checked="isDontShow"
            :on-check="toggleDontShow"
            label="I understand, don’t show this again."
        />
        <ButtonsContainer>
            <template #leftButton>
                <VButton
                    label="Back"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="backClick"
                    :is-white="true"
                />
            </template>
            <template #rightButton>
                <VButton
                    label="Continue ->"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="continueClick"
                />
            </template>
        </ButtonsContainer>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { LocalData } from '@/utils/localData';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import ButtonsContainer from '@/components/accessGrants/createFlow/components/ButtonsContainer.vue';
import Toggle from '@/components/accessGrants/createFlow/components/Toggle.vue';
import VButton from '@/components/common/VButton.vue';

const props = defineProps<{
    onBack: () => void;
    onContinue: () => void;
}>();

const analyticsStore = useAnalyticsStore();

const isDontShow = ref<boolean>(false);

/**
 * Sends "trackPageVisit" event to segment.
 */
function trackPageVisit(): void {
    analyticsStore.pageVisit('https://docs.storj.io/dcs/concepts/encryption-key/design-decision-server-side-encryption/');
}

/**
 * Toggles 'passphrase is saved' checkbox.
 */
function toggleDontShow(): void {
    isDontShow.value = !isDontShow.value;
}

/**
 * Holds on continue click button logic.
 */
function continueClick(): void {
    if (isDontShow.value) {
        LocalData.setServerSideEncryptionModalHidden(true);
    }
    props.onContinue();
}

/**
 * Holds on back click button logic.
 */
function backClick(): void {
    if (isDontShow.value) {
        LocalData.setServerSideEncryptionModalHidden(true);
    }
    props.onBack();
}
</script>

<style lang="scss" scoped>
.warning-step {
    font-family: 'font_regular', sans-serif;

    &__info {
        font-size: 16px;
        line-height: 24px;
        color: var(--c-blue-6);
        padding: 16px 0;
        text-align: left;

        &__link {
            font-size: 16px;
            line-height: 24px;
            color: var(--c-blue-6);
            text-decoration: underline !important;
            text-underline-position: under;
        }
    }
}
</style>
