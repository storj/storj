// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert v-if="shouldShow" border class="my-4 pb-5" variant="outlined" :color="expired ? 'error' : 'warning'" :title="title" closable>
        <p class="text-body-medium mt-2 mb-4">
            {{ info }} <span v-if="configStore.billingEnabled">Upgrade to continue using {{ configStore.brandName }} for your own projects.</span><br>
            <template v-if="projectInvitationsEnabled"><strong>Note:</strong> You will continue to maintain access to projects that you are a member of.</template>
        </p>
        <v-btn
            v-if="configStore.billingEnabled"
            :color="expired ? 'error' : 'warning'"
            :prepend-icon="CircleArrowUp"
            @click="onUpgrade"
        >
            Upgrade
        </v-btn>
        <v-btn
            v-else
            :color="expired ? 'primary' : 'warning'"
            :append-icon="ArrowRight"
            variant="flat"
            link
            target="_blank"
            :href="configStore.supportUrl"
            rel="noopener noreferrer"
        >
            Contact Support
        </v-btn>
    </v-alert>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VAlert, VBtn } from 'vuetify/components';
import { ArrowRight, CircleArrowUp } from '@lucide/vue';

import type { ExpirationInfo } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';

const usersStore = useUsersStore();
const configStore = useConfigStore();
const appStore = useAppStore();

const props = withDefaults(defineProps<{
    expired?: boolean
}>(), {
    expired: false,
});

/**
 * Returns dialog title based on expired status.
 */
const title = computed<string>(() => {
    return props.expired ? 'Trial Expired' : 'Your Trial is Expiring Soon';
});

const projectInvitationsEnabled = computed<boolean>(() => configStore.state.config.projectInvitationsEnabled);

const expirationInfo = computed<ExpirationInfo>(() => usersStore.state.user.getExpirationInfo(configStore.state.config.daysBeforeTrialEndNotification));

const shouldShow = computed<boolean>(() => {
    if (props.expired) return true;
    const expiration = usersStore.state.user.trialExpiration;
    return !!expiration && expiration.getTime() > Date.now();
});

const info = computed<string>(() => {
    const days = expirationInfo.value.days;
    if (props.expired) {
        return days === 0 ? 'Your trial expired less than a day ago.' : `Your trial expired ${days} day${days === 1 ? '' : 's'} ago.`;
    }
    return days === 0 ? 'Less than a day left in your trial.' : `Only ${days} day${days === 1 ? '' : 's'} left in your trial.`;
});

/**
 * Starts upgrade account flow.
 */
function onUpgrade(): void {
    if (!configStore.billingEnabled) return;

    appStore.toggleUpgradeFlow(true);
}
</script>
