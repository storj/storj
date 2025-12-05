// Copyright (C) 2024 Storj Labs, Inc.
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
                        <component :is="LockKeyhole" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">
                    {{ userSettings.passphrasePrompt ? 'Disable' : 'Enable' }} Passphrase Prompt
                </v-card-title>
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
            <v-card-item class="px-6 py-5">
                <p v-if="userSettings.passphrasePrompt">
                    If you proceed, you will only be prompted to enter your project encryption passphrase when it is
                    necessary (e.g. opening a bucket, creating an access grant)
                </p>
                <p v-else>
                    If you proceed, you will be prompted to enter your project encryption passphrase as soon as you
                    open a project, so that you do not need to enter it later
                </p>
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
                            @click="togglePassphrasePrompt"
                        >
                            Continue
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
    VCard,
    VCardItem,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VSheet,
} from 'vuetify/components';
import { LockKeyhole, X } from 'lucide-vue-next';

import { useUsersStore } from '@/store/modules/usersStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { UserSettings } from '@/types/users';
import { useProjectsStore } from '@/store/modules/projectsStore';

const usersStore = useUsersStore();
const projectsStore = useProjectsStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

/**
 * Returns user settings from store.
 */
const userSettings = computed((): UserSettings => {
    return usersStore.state.settings as UserSettings;
});

/**
 * Handles toggling passphrase prompt setting.
 */
async function togglePassphrasePrompt(): Promise<void> {
    await withLoading(async () => {
        const newPassphrasePrompt = !userSettings.value.passphrasePrompt;
        try {
            await usersStore.updateSettings({
                passphrasePrompt: newPassphrasePrompt,
            });
            notify.success(`Passphrase prompt ${userSettings.value.passphrasePrompt ? 'enabled' : 'disabled'} successfully`);
            model.value = false;
            if (newPassphrasePrompt) {
                // deselect current project so user is prompted for passphrase when
                // they open it again.
                projectsStore.deselectProject();
            }
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETTINGS_AREA);
        }
    });
}
</script>
