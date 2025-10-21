// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        :persistent="isLoading"
        width="auto"
        max-width="420px"
        transition="fade-transition"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="Timer" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Session Timeout</v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>
            <v-divider />
            <v-card-item class="px-6 pt-6 pb-2">
                <p>Select your account session timeout duration.</p>
            </v-card-item>
            <v-card-item class="px-6 pb-7">
                <v-select
                    v-model="duration"
                    class="pt-2"
                    :items="options"
                    variant="outlined"
                    item-title="shortString"
                    item-value="nanoseconds"
                    label="Session timeout duration"
                    return-object
                    hide-details
                />
            </v-card-item>
            <v-divider />
            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :loading="isLoading"
                            @click="onChangeTimeout"
                        >
                            Save
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VSelect,
    VSheet,
} from 'vuetify/components';
import { Timer, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/modules/usersStore';
import { Duration } from '@/utils/time';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';

const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const options = [
    Duration.MINUTES_15,
    Duration.MINUTES_30,
    Duration.HOUR_1,
    Duration.DAY_1,
    Duration.WEEK_1,
    Duration.DAY_30,
];

const model = defineModel<boolean>({ required: true });

/**
 * Returns user's session duration from store.
 */
const storedDuration = computed((): Duration | null => {
    return usersStore.state.settings.sessionDuration;
});

const duration = ref<Duration>(storedDuration.value || Duration.MINUTES_15);

/**
 * Handles change session timeout request.
 */
async function onChangeTimeout(): Promise<void> {
    await withLoading(async () => {
        try {
            await usersStore.updateSettings({ sessionDuration: duration.value.nanoseconds });
            notify.success(`Session timeout changed successfully. Your session timeout is ${duration.value?.shortString}.`);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.EDIT_TIMEOUT_MODAL);
            return;
        }

        model.value = false;
    });
}
</script>
