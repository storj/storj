// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert border class="my-4 pb-5" variant="outlined" :color="expired ? 'error' : 'warning'" :title="title" closable>
        <p class="text-body-2 mt-2 mb-4">
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
import { ArrowRight, CircleArrowUp } from 'lucide-vue-next';

import { ExpirationInfo } from '@/types/users';
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

/**
 * Returns expiration info based on expired status.
 */
const info = computed<string>(() => {
    return props.expired ? `Your trial expired ${expirationInfo.value.days} days ago.` : `Only ${expirationInfo.value.days} days left in your trial.`;
});

/**
 * Returns user free trial expiration info.
 */
const expirationInfo = computed<ExpirationInfo>(() => usersStore.state.user.getExpirationInfo(configStore.state.config.daysBeforeTrialEndNotification));

/**
 * Starts upgrade account flow.
 */
function onUpgrade(): void {
    if (!configStore.billingEnabled) return;

    appStore.toggleUpgradeFlow(true);
}
</script>
