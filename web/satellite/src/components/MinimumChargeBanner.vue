// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        v-model="alertVisible"
        closable
        variant="outlined"
        type="info"
        class="my-4 pb-4"
        border
    >
        <template #title>
            <p>Update: Minimum Monthly Usage Fee</p>
        </template>
        <p class="my-1">
            Starting {{ minimumCharge.monthYearStartDateStr }}, Storj will apply a {{ minimumCharge.amount }}
            minimum monthly usage fee if your usage is less than {{ minimumCharge.amount }} per month.
        </p>

        <p>
            <a class="link" href="https://storj.dev/dcs/pricing#minimum-monthly-billing" target="_blank">Learn more</a>
            <template v-if="minimumCharge.startDate && minimumCharge.startDate > new Date()">
                or <a class="link" href="https://storj.dev/support/account-management-billing/closing-an-account" target="_blank">close your account</a>
                by {{ minimumCharge.longStartDateStr }} if you prefer not to continue.
            </template>
        </p>
    </v-alert>
</template>

<script setup lang="ts">
import { VAlert } from 'vuetify/components';
import { computed } from 'vue';

import { MinimumCharge, useConfigStore } from '@/store/modules/configStore';

const configStore = useConfigStore();

const alertVisible = computed<boolean>({
    get: () => !configStore.minimumChargeBannerDismissed && configStore.minimumCharge.priorNoticeEnabled,
    set: (value: boolean) => configStore.minimumChargeBannerDismissed = !value,
});

const minimumCharge = computed<MinimumCharge>(() => {
    return configStore.minimumCharge;
});
</script>
<style scoped lang="scss">
p {
    color: rgb(var(--v-theme-on-background));
}
</style>
