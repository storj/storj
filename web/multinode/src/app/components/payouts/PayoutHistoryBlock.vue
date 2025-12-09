// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <info-block>
        <template #body>
            <div class="payouts-history-block">
                <h3 class="payouts-history-block__title">Payout History</h3>
                <v-button class="payouts-history-block__button" label="Download" :is-white="true" width="100%" />
            </div>
        </template>
    </info-block>
</template>

<script setup lang="ts">
import InfoBlock from '@/app/components/common/InfoBlock.vue';
import VButton from '@/app/components/common/VButton.vue';
</script>

<style scoped lang="scss">
    .payouts-history-block {
        box-sizing: border-box;
        width: 100%;
        display: flex;
        flex-direction: column;
        align-items: flex-start;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            color: var(--c-title);
            margin-bottom: 30px;
        }

        &__button {
            background-color: var(--c-button-gray);
            border: none !important;
        }
    }
</style>
