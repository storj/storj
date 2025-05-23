// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        v-if="alertVisible"
        closable
        variant="outlined"
        type="info"
        class="my-4 pb-4"
        :title="title"
        border
    >
        <p class="my-1">
            {{ message }}
        </p>

        <p>
            <a href="https://storj.dev/dcs/pricing#minimum-monthly-billing" target="_blank">Learn more</a>
            , <a href="https://forum.storj.io" target="_blank">join the discussion</a>
            <template v-if="minimumCharge.startDate && minimumCharge.startDate > new Date()">
                , or <a href="https://storj.dev/support/account-management-billing/closing-an-account" target="_blank">close your account</a>
                by {{ minimumCharge.shortStartDateStr }} if you prefer not to continue.
            </template>
        </p>
    </v-alert>
</template>

<script setup lang="ts">
import { VAlert } from 'vuetify/components';
import { computed, ref } from 'vue';

import { MinimumCharge, useConfigStore } from '@/store/modules/configStore';

const configStore = useConfigStore();

const dismissed = ref(false);

const alertVisible = computed<boolean>(() => {
    if (dismissed.value) return false;
    if (!minimumCharge.value.enabled || !minimumCharge.value.startDate) return false;

    const currentDate = new Date();
    const startDate = minimumCharge.value.startDate;
    const thirtyDaysBefore = new Date(startDate);
    thirtyDaysBefore.setDate(thirtyDaysBefore.getDate() - 30);
    const forty5DaysAfter = new Date(startDate);
    forty5DaysAfter.setDate(forty5DaysAfter.getDate() + 45);

    return currentDate >= thirtyDaysBefore && currentDate <= forty5DaysAfter;
});

const minimumCharge = computed<MinimumCharge>(() => {
    return configStore.minimumCharge;
});

const title = computed<string>(() => {
    const isAfterStartDate = new Date() >= minimumCharge.value.startDate!;

    return `Minimum Usage Fee ${isAfterStartDate ? 'Started' : 'Starts'} ${minimumCharge.value.shortStartDateStr}`;
});

const message = computed<string>(() => {
    const isAfterStartDate = new Date() >= minimumCharge.value.startDate!;

    return `Starting ${minimumCharge.value.longStartDateStr}, Storj ${isAfterStartDate? 'applies' : 'will apply'} a ${minimumCharge.value.amount}
     minimum monthly usage fee. If your monthly usage already exceeds ${minimumCharge.value.amount}, 
     no additional fees will apply.`;
});
</script>
