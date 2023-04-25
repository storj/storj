// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <UpgradeAccountWrapper title="Upgrade to Pro">
        <template #content>
            <p class="options-info">
                Add a credit card to activate your Pro Account, or deposit more than $10 in STORJ tokens to upgrade
                and get 10% bonus on your STORJ tokens deposit.
            </p>
            <div class="options-buttons">
                <VButton
                    label="Add Credit Card"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    border-radius="10px"
                    icon="credit-card"
                    :is-green="true"
                    :on-press="onAddCard"
                    :is-disabled="loading"
                />
                <VButton
                    label="Add STORJ Tokens"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    border-radius="10px"
                    icon="addcircle"
                    :on-press="onAddTokens"
                    :is-disabled="loading"
                />
            </div>
        </template>
    </UpgradeAccountWrapper>
</template>

<script setup lang="ts">
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';

import UpgradeAccountWrapper from '@/components/modals/upgradeAccountFlow/UpgradeAccountWrapper.vue';
import VButton from '@/components/common/VButton.vue';

const usersStore = useUsersStore();
const notify = useNotify();

const props = defineProps<{
    loading: boolean;
    onAddCard: () => void;
    onAddTokens: () => Promise<void>;
}>();
</script>

<style scoped lang="scss">
.options-info {
    font-family: 'font_regular', sans-serif;
    font-size: 14px;
    line-height: 20px;
    color: var(--c-blue-6);
    padding-bottom: 16px;
    border-bottom: 1px solid var(--c-grey-2);
    margin-bottom: 16px;
    max-width: 400px;
    text-align: left;
}

.options-buttons {
    display: flex;
    align-items: center;
    column-gap: 16px;

    @media screen and (max-width: 520px) {
        flex-direction: column;
        column-gap: unset;
        row-gap: 16px;
    }
}
</style>
