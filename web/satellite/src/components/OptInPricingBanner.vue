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
        :color="preFreezeColor"
        :title="preFreezeTitle"
        class="my-4 pb-4"
        border
    >
        <p class="text-body-medium mt-2 mb-4">
            <template v-if="mayBeFrozen">
                You haven't accepted the updated pricing. To continue using {{ configStore.brandName }},
                please review and accept the new plan.
            </template>
            <template v-else>
                You haven't accepted the updated pricing. Review the updated plan and choose
                whether to continue using {{ configStore.brandName }}.
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
        <v-btn
            :color="preFreezeColor" density="comfortable" @click="showOptInPopup"
        >
            Review Pricing
        </v-btn>
    </v-alert>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VAlert, VBtn } from 'vuetify/components';

import { OptInStatus } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';
import { formatConfigDate, freezeDateInFuture } from '@/types/pricingOptIn';

const appStore = useAppStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();

const preFreezeVisible = ref(true);

const optInStatus = computed<OptInStatus>(() => usersStore.state.settings.optInStatus);
const freezeStatus = computed(() => usersStore.state.user.freezeStatus);
const freezeDateLabel = computed<string>(() => formatConfigDate(configStore.state.config.optOutFreezeDate));
const freezeInFuture = computed<boolean>(() => freezeDateInFuture(configStore.state.config.optOutFreezeDate));

/**
 * Whether the user may be frozen if they don't opt in. If optOutFreezeOptedOutOnly is true,
 * only OptedOut users get frozen. Otherwise, both OptedOut and NoAction get frozen.
*/
const mayBeFrozen = computed<boolean>(() =>
    !!configStore.state.config.optOutFreezeDate
    && (!configStore.state.config.optOutFreezeOptedOutOnly || optInStatus.value === OptInStatus.OptedOut),
);

const showSuspended = computed<boolean>(() => freezeStatus.value.optOutFrozen && configStore.state.config.optInPopupEnabled);

/**
 * Whether to still show the pre-freeze state if the user is not frozen regardless of whether
 * the freeze date has passed.
*/
const showPreFreeze = computed<boolean>(() => {
    if (!configStore.state.config.optInPopupEnabled || showSuspended.value) return false;
    return optInStatus.value !== OptInStatus.OptedIn && optInStatus.value !== OptInStatus.Excluded;
});

const preFreezeTitle = computed<string>(() => {
    if (!mayBeFrozen.value) return 'Review the updated pricing';
    if (freezeInFuture.value) return `Your account will be restricted on ${freezeDateLabel.value}.`;
    return 'Your account is scheduled to be restricted.';
});

const preFreezeColor = computed<string>(() => mayBeFrozen.value ? 'warning' : 'info');

const suspendedDays = computed<number>(() => freezeStatus.value.optOutGracePeriod);

function showOptInPopup() {
    if (configStore.state.config.optInPopupEnabled) {
        appStore.togglePricingOptInDialog(true);
    }
}
</script>
