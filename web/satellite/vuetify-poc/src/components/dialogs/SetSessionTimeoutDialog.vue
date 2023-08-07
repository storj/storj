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
            <v-card-item class="pl-7 pr-0 pb-5 pt-0">
                <v-row align="start" justify="space-between" class="ma-0">
                    <v-row align="center" class="ma-0 pt-5">
                        <img class="flex-shrink-0" src="@poc/assets/icon-session-timeout.svg" alt="Change name">
                        <v-card-title class="font-weight-bold ml-4">Session Timeout</v-card-title>
                    </v-row>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </v-row>
            </v-card-item>
            <v-divider />
            <v-card-item class="px-7 py-5">
                <p>Select your session timeout duration.</p>
            </v-card-item>
            <v-divider />
            <v-card-item class="px-7 py-5">
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
            <v-card-actions class="px-7 py-5">
                <v-row class="ma-0">
                    <v-col class="pl-0">
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
                    <v-col class="pr-0">
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

const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();

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
        } catch (error) {
            return;
        }

        emit('update:modelValue', false);
    });
}
</script>
