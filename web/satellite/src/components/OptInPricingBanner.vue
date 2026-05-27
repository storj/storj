// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        v-if="showSuspended"
        variant="outlined"
        color="error"
        title="Your account is suspended."
        class="my-4 pb-4"
        border
    >
        <p class="text-body-medium mt-2 mb-4">
            <template v-if="suspendedDays === 0">
                Your account is marked for deletion. Restore it by accepting the updated pricing.
            </template>
            <template v-else>
                Your account will be marked for deletion in {{ suspendedDays }} day{{ suspendedDays === 1 ? '' : 's' }}.
                Accept the updated pricing or export your data before this deadline.
            </template>
            <a
                class="link"
                href="https://storj.dev/dcs/pricing"
                target="_blank"
                rel="noopener noreferrer"
            >
                Learn More
            </a>.
        </p>
        <v-btn color="error" density="comfortable" @click="showOptInPopup">
            Restore Account
        </v-btn>
    </v-alert>

    <v-alert
        v-else-if="showPreFreeze"
        v-model="preFreezeVisible"
        closable
        variant="outlined"
        color="warning"
        :title="`Your account will be restricted on ${freezeDateLabel}.`"
        class="my-4 pb-4"
        border
    >
        <p class="text-body-medium mt-2 mb-4">
            You haven't accepted the updated pricing. To continue using {{ configStore.brandName }},
            please review and accept the new plan.
            <a
                class="link"
                href="https://storj.dev/dcs/pricing"
                target="_blank"
                rel="noopener noreferrer"
            >
                Learn More
            </a>.
        </p>
        <v-btn
            color="warning" density="comfortable" @click="showOptInPopup"
        >
            Review Pricing
        </v-btn>
    </v-alert>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VAlert, VBtn } from 'vuetify/components';
import { useDate } from 'vuetify';

import { OptInStatus } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';

const appStore = useAppStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();

const date = useDate();

const preFreezeVisible = ref(true);

const optInStatus = computed<OptInStatus>(() => usersStore.state.settings.optInStatus);
const freezeStatus = computed(() => usersStore.state.user.freezeStatus);
const freezeDate = computed(() => date.date(configStore.state.config.optOutFreezeDate));
const freezeDateLabel = computed(() => date.format(freezeDate.value, 'monthAndDate'));

const showSuspended = computed<boolean>(() => freezeStatus.value.optOutFrozen && configStore.state.config.optInPopupEnabled);

const showPreFreeze = computed<boolean>(() => {
    if (!configStore.state.config.optInPopupEnabled || showSuspended.value) return false;
    if (optInStatus.value === OptInStatus.OptedIn || optInStatus.value === OptInStatus.Excluded) return false;
    return date.isBefore(new Date(), freezeDate.value);
});

const suspendedDays = computed<number>(() => freezeStatus.value.optOutGracePeriod);

function showOptInPopup() {
    if (configStore.state.config.optInPopupEnabled) {
        appStore.togglePricingOptInDialog(true);
    }
}
</script>
