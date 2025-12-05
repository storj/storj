// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card>
            <v-sheet>
                <v-card-item class="py-4 pl-6">
                    <template #prepend>
                        <img v-if="!expired" src="@/assets/icon-trial-expiring.svg" alt="Trial Icon" width="40" class="mt-1">
                    </template>
                    <v-card-title class="font-weight-bold">{{ title }}</v-card-title>
                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-row>
                <v-col v-if="!expired || !trialExpirationGracePeriod" class="pa-6 mx-3">
                    <p class="text-body-2 font-weight-bold mb-2" />
                    <v-chip variant="tonal" :color="expired ? 'error' : 'warning'" class="font-weight-bold">{{ info }}</v-chip>
                    <p class="text-body-2 my-2">Upgrade your account to {{ expired ? 'continue' : 'keep' }} using Storj.</p>
                </v-col>
                <v-col v-else class="pa-6 mx-3">
                    <p class="text-body-2 my-2">
                        We hope you enjoyed your trial! Your account is currently inactive,
                        but there's still time to continue using Storj. Upgrade now to keep
                        your data and access all features.
                    </p>
                    <p class="text-body-2 my-2 font-weight-bold">
                        Your account will be scheduled for deletion in
                        {{ trialExpirationGracePeriod }} if no action is taken.
                    </p>
                </v-col>
            </v-row>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            :color="expired ? 'primary' : 'warning'"
                            :append-icon="ArrowRight"
                            variant="flat"
                            block
                            @click="onUpgrade"
                        >
                            Upgrade Now
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VDialog,
    VSheet,
    VCard,
    VCardItem,
    VCardTitle,
    VCardActions,
    VRow,
    VCol,
    VDivider,
    VChip,
    VBtn,
} from 'vuetify/components';
import { ArrowRight, X } from 'lucide-vue-next';

import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ExpirationInfo } from '@/types/users';
import { useAppStore } from '@/store/modules/appStore.js';

const props = withDefaults(defineProps<{
    expired?: boolean
}>(), {
    expired: false,
});

const usersStore = useUsersStore();
const configStore = useConfigStore();
const appStore = useAppStore();

const model = defineModel<boolean>({ required: true });

/**
 * Returns how many days until user is marked for deletion.
 */
const trialExpirationGracePeriod = computed<string>(() => {
    const days = usersStore.state.user.freezeStatus?.trialExpirationGracePeriod ?? 0;
    if (days <= 0) {
        return '';
    }
    return `${days} day${days > 1 ? 's' : ''}`;
});

/**
 * Returns user free trial expiration info.
 */
const expirationInfo = computed<ExpirationInfo>(() => usersStore.state.user.getExpirationInfo(configStore.state.config.daysBeforeTrialEndNotification));

/**
 * Returns dialog title based on expired status.
 */
const title = computed<string>(() => {
    return props.expired ? 'Your Trial Has Expired' : 'Trial Expiring Soon';
});

/**
 * Returns expiration info based on expired status.
 */
const info = computed<string>(() => {
    const prefix = 'Your trial account';
    return props.expired ? `${prefix} expired ${expirationInfo.value.days} days ago.` : `${prefix} expires in ${expirationInfo.value.days} days.`;
});

/**
 * Starts upgrade account flow.
 */
function onUpgrade(): void {
    model.value = false;
    appStore.toggleUpgradeFlow(true);
}
</script>
