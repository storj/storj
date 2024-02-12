// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="410px"
        transition="fade-transition"
        persistent
    >
        <v-card rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <template #prepend>
                    <img class="d-block" src="@/assets/icon-session-timeout.svg" alt="Session expired">
                </template>
                <v-card-title class="font-weight-bold">Session timed out?</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        @click="onLeaveAsIs"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item class="pa-8">
                Your last session was logged out due to inactivity. Did you know you can update your preferred session timeout?
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="onLeaveAsIs">
                            Don't update timeout
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn variant="flat" color="primary" block @click="onUpdate">
                            Update timeout
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { watch } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VBtn,
    VDivider,
    VCardActions,
    VCol,
    VRow,
} from 'vuetify/components';

import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { LocalData } from '@/utils/localData';

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    'showSetTimeoutModal': [],
}>();

const usersStore = useUsersStore();
const configStore = useConfigStore();

/**
 * Starts update session timeout flow.
 */
function onUpdate(): void {
    model.value = false;
    emit('showSetTimeoutModal');
}

/**
 * Changes session timeout value to default one.
 */
function onLeaveAsIs(): void {
    usersStore.updateSettings({ sessionDuration: configStore.state.config.inactivityTimerDuration * 1000000000 }); // nanoseconds
    model.value = false;
}

watch(model, (value: boolean) => {
    if (value) LocalData.removeSessionHasExpired();
});
</script>
