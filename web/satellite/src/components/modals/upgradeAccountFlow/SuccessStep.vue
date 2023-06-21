// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <UpgradeAccountWrapper title="Success" :icon="SuccessIcon">
        <template #content>
            <p class="success-info">Your Pro Account has been successfully activated.</p>
            <VButton
                label="Continue ->"
                width="100%"
                height="48px"
                font-size="14px"
                border-radius="10px"
                :is-green="true"
                :on-press="onContinue"
            />
        </template>
    </UpgradeAccountWrapper>
</template>

<script setup lang="ts">
import UpgradeAccountWrapper from '@/components/modals/upgradeAccountFlow/UpgradeAccountWrapper.vue';
import VButton from '@/components/common/VButton.vue';

import SuccessIcon from '@/../static/images/modals/upgradeFlow/success.svg';

const props = defineProps<{
    onContinue: () => void;
}>();
</script>

<style scoped lang="scss">
.success-info {
    font-family: 'font_regular', sans-serif;
    font-size: 14px;
    line-height: 20px;
    color: var(--c-blue-6);
    padding-bottom: 16px;
    border-bottom: 1px solid var(--c-grey-2);
    margin-bottom: 16px;
}
</style>
