// Copyright (C) 2023 Storj Labs, Inc.
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
                        <v-icon :icon="LockKeyhole" size="18" />
                    </v-sheet>
                </template>

                <v-card-title class="font-weight-bold">
                    Enter passphrase
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
                            <p>
                                Enter your encryption passphrase to view and manage the data in this project.
                            </p>
                        </v-col>

                        <v-col cols="12">
                            <v-text-field
                                id="Encryption Passphrase"
                                v-model="passphrase"
                                :base-color="isWarningState ? 'warning' : ''"
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

                <v-alert
                    v-if="isWarningState"
                    class="mt-3"
                    density="compact"
                    type="warning"
                    text="Object count mismatch: objects may be uploaded with a different passphrase, or objects have been recently deleted and are not reflected yet."
                />
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-3">
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
                        :color="isWarningState ? 'default' : 'primary'"
                        :variant="isWarningState ? 'outlined' : 'flat'"
                        block
                        :loading="isLoading"
                        :disabled="!formValid"
                        @click="onContinue"
                    >
                        Continue ->
                    </v-btn>
                </v-col>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, ref, watch } from 'vue';
import { VForm, VRow, VCol, VTextField, VCardItem, VDivider, VCardTitle, VBtn, VCard, VCardActions, VDialog, VAlert, VSheet, VIcon } from 'vuetify/components';
import { LockKeyhole, X } from 'lucide-vue-next';

import { RequiredRule } from '@/types/common';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { Bucket } from '@/types/buckets';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';

import PasswordInputEyeIcons from '@/components/PasswordInputEyeIcons.vue';

const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const passphrase = ref<string>('');
const isPassphraseVisible = ref<boolean>(false);
const isWarningState = ref<boolean>(false);
const innerContent = ref<Component | null>(null);
const formValid = ref<boolean>(false);
const isLoading = ref<boolean>(false);

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    (event: 'passphraseEntered'): void,
}>();

/**
 * Returns chosen bucket name from store.
 */
const bucketName = computed((): string => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Returns selected bucket name object count.
 */
const bucketObjectCount = computed((): number => {
    const data: Bucket | undefined = bucketsStore.state.page.buckets.find(
        (bucket: Bucket) => bucket.name === bucketName.value,
    );

    return data?.objectCount ?? 0;
});

/**
 * Sets access and navigates to object browser.
 */
async function onContinue(): Promise<void> {
    if (isLoading.value) return;

    if (isWarningState.value) {
        bucketsStore.setPromptForPassphrase(false);

        model.value = false;
        emit('passphraseEntered');

        return;
    }

    isLoading.value = true;

    try {
        bucketsStore.setPassphrase(passphrase.value);
        await bucketsStore.setS3Client(projectsStore.state.selectedProject.id);
        const count: number = await bucketsStore.getObjectsCount(bucketName.value);
        if (count === 0 && bucketObjectCount.value > 0) {
            isWarningState.value = true;
            isLoading.value = false;
            return;
        }
        bucketsStore.setPromptForPassphrase(false);
        isLoading.value = false;
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.OPEN_BUCKET_MODAL);
        isLoading.value = false;
        return;
    }

    model.value = false;
    emit('passphraseEntered');
}

watch(innerContent, comp => {
    if (!comp) {
        passphrase.value = '';
        isWarningState.value = false;
    }
});

watch(passphrase, () => {
    if (isWarningState.value) isWarningState.value = false;
});
</script>
