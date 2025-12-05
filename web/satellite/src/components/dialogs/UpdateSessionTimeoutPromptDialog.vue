// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        persistent
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
                <v-card-title class="font-weight-bold">Session timed out?</v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        @click="onLeaveAsIs"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item class="pa-6">
                Your last session was logged out due to inactivity. Did you know you can update your preferred session timeout?
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
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
    VSheet,
} from 'vuetify/components';
import { Timer, X } from 'lucide-vue-next';

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
