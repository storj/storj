// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        :persistent="isLoading"
        max-width="420px"
        transition="fade-transition"
    >
        <v-card ref="innerContent">
            <v-card-item class="pa-6 pr-5">
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
                    {{ isSkipping ? 'Skip passphrase' : 'Enter passphrase' }}
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

            <v-card-item class="pa-6 pb-3">
                <v-form v-model="formValid" @submit.prevent="onContinue">
                    <v-row>
                        <v-col cols="12">
                            <p v-if="isSkipping" class="pb-3">
                                Do you want to remember this choice and skip the passphrase when opening a project?
                            </p>
                            <p v-else>
                                Enter your encryption passphrase to view and manage the data in this project.
                            </p>
                        </v-col>

                        <v-col v-if="!isSkipping" cols="12">
                            <v-text-field
                                id="Encryption Passphrase"
                                v-model="passphrase"
                                label="Encryption Passphrase"
                                :type="isPassphraseVisible ? 'text' : 'password'"
                                variant="outlined"
                                :hide-details="false"
                                :rules="[ RequiredRule ]"
                                autofocus
                                required
                            >
                                <template #append-inner>
                                    <password-input-eye-icons
                                        :is-visible="isPassphraseVisible"
                                        type="passphrase"
                                        @toggle-visibility="isPassphraseVisible = !isPassphraseVisible"
                                    />
                                </template>
                            </v-text-field>
                        </v-col>
                    </v-row>
                </v-form>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-3">
                <v-col>
                    <v-btn
                        variant="outlined"
                        color="default"
                        block
                        :disabled="isLoading"
                        @click="() => isSkipping ? model = false : onSkip()"
                    >
                        {{ isSkipping ? 'No, skip just now' : 'Skip' }}
                    </v-btn>
                </v-col>
                <v-col>
                    <v-btn
                        color="primary"
                        variant="flat"
                        block
                        :loading="isLoading"
                        :disabled="!formValid"
                        @click="() => isSkipping ? onSkip(true) : onContinue()"
                    >
                        {{ isSkipping ? 'Yes, always skip' : 'Continue ->' }}
                    </v-btn>
                </v-col>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VRow,
    VTextField,
    VSheet,
} from 'vuetify/components';
import { LockKeyhole, X } from 'lucide-vue-next';

import { RequiredRule } from '@/types/common';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useAppStore } from '@/store/modules/appStore';
import { useLoading } from '@/composables/useLoading';
import {
    AnalyticsErrorEventSource,
    AnalyticsEvent,
} from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/modules/usersStore.js';
import { useProjectsStore } from '@/store/modules/projectsStore';

import PasswordInputEyeIcons from '@/components/PasswordInputEyeIcons.vue';

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const projectStore = useProjectsStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const passphrase = ref<string>('');
const isPassphraseVisible = ref<boolean>(false);
const isSkipping = ref<boolean>(false);
const innerContent = ref<VCard | null>(null);
const formValid = ref<boolean>(false);

const model = computed({
    get: () => appStore.state.isProjectPassphraseDialogShown,
    set: appStore.toggleProjectPassphraseDialog,
});

function onSkip(confirmed = false): void {
    if (!confirmed) {
        isSkipping.value = true;
        return;
    }

    withLoading(async () => {
        try {
            await usersStore.updateSettings({ passphrasePrompt: false });
            model.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CREATE_PROJECT_PASSPHRASE_MODAL);
        }
    });
}

async function onContinue(): Promise<void> {
    analyticsStore.eventTriggered(AnalyticsEvent.PASSPHRASE_CREATED, {
        method: 'enter',
        project_id: projectStore.state.selectedProject.id,
    });

    bucketsStore.setPassphrase(passphrase.value);
    bucketsStore.setPromptForPassphrase(false);

    model.value = false;
}

watch(innerContent, comp => {
    if (!comp) passphrase.value = '';
});
</script>
