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
                        <img v-if="expired" src="@/assets/icon-trial-expired.svg" alt="Trial Icon" width="40" class="mt-1">
                        <img v-else src="@/assets/icon-trial-expiring.svg" alt="Trial Icon" width="40" class="mt-1">
                    </template>
                    <v-card-title class="font-weight-bold">{{ title }}</v-card-title>
                    <template #append>
                        <v-btn
                            icon="$close"
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
                <v-col class="pa-6 mx-3">
                    <p class="text-body-2 font-weight-bold mb-2" />
                    <v-chip variant="tonal" :color="expired ? 'error' : 'warning'" class="font-weight-bold">{{ info }}</v-chip>
                    <p class="text-body-2 my-2">Upgrade your account to {{ expired ? 'continue' : 'keep' }} using Storj.</p>
                </v-col>
            </v-row>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn :color="expired ? 'error' : 'warning'" variant="flat" block @click="onUpgrade">
                            Go To Upgrade<v-icon :icon="mdiArrowRight" class="ml-1" />
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
    VIcon,
} from 'vuetify/components';
import { mdiArrowRight } from '@mdi/js';

import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ExpirationInfo } from '@/types/users';
import { useAppStore } from '@/store/modules/appStore.js';

const props = withDefaults(defineProps<{
    expired: boolean
}>(), {
    expired: false,
});

const usersStore = useUsersStore();
const configStore = useConfigStore();
const appStore = useAppStore();

const model = defineModel<boolean>({ required: true });

/**
 * Returns user free trial expiration info.
 */
const expirationInfo = computed<ExpirationInfo>(() => usersStore.state.user.getExpirationInfo(configStore.state.config.daysBeforeTrialEndNotification));

/**
 * Returns dialog title based on expired status.
 */
const title = computed<string>(() => {
    return props.expired ? 'Trial Expired' : 'Trial Expiring Soon';
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
