// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="320px"
        max-width="410px"
        transition="fade-transition"
    >
        <v-card rounded="xlg">
            <v-card-item class="pl-6 py-4">
                <template #prepend>
                    <img class="d-block" src="@poc/assets/icon-session-timeout.svg" alt="Session timeout">
                </template>
                <v-card-title class="font-weight-bold">Session Timeout</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>
            <v-divider />
            <v-card-item class="px-6 pt-6 pb-3">
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
                            :loading="isLoading"
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
                            :disabled="isLoading"
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
} from 'vuetify/components';

import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/modules/usersStore';
import { Duration } from '@/utils/time';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';

const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    modelValue: boolean,
}>();

const emit = defineEmits<{
    (event: 'update:modelValue', value: boolean): void,
}>();

const options = [
    Duration.MINUTES_15,
    Duration.MINUTES_30,
    Duration.HOUR_1,
    Duration.DAY_1,
    Duration.WEEK_1,
    Duration.DAY_30,
];

/**
 * Returns user's session duration from store.
 */
const storedDuration = computed((): Duration | null => {
    return usersStore.state.settings.sessionDuration;
});

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
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

        emit('update:modelValue', false);
    });
}
</script>
