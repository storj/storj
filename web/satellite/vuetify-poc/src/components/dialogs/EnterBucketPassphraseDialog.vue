// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="400px"
        transition="fade-transition"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <template #prepend>
                    <img class="d-block" src="@/../static/images/accessGrants/newCreateFlow/accessEncryption.svg">
                </template>

                <v-card-title class="font-weight-bold">
                    Enter passphrase
                </v-card-title>

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

            <v-card-item class="pa-7 pb-3">
                <v-form v-model="formValid" @submit.prevent="onContinue">
                    <v-row>
                        <v-col cols="12">
                            <p>
                                Enter your encryption passphrase to view and manage the data in this project.
                            </p>
                        </v-col>

                        <v-col cols="12">
                            <v-text-field
                                v-model="passphrase"
                                :base-color="isWarningState ? 'warning' : ''"
                                label="Encryption Passphrase"
                                :append-inner-icon="isPassphraseVisible ? 'mdi-eye-outline' : 'mdi-eye-off-outline'"
                                :type="isPassphraseVisible ? 'text' : 'password'"
                                variant="outlined"
                                :hide-details="false"
                                :rules="[ RequiredRule ]"
                                autofocus
                                @click:append-inner="isPassphraseVisible = !isPassphraseVisible"
                            />
                        </v-col>
                    </v-row>
                </v-form>

                <v-alert
                    v-if="isWarningState"
                    class="mt-3"
                    density="compact"
                    type="warning"
                    text="This bucket includes files that are uploaded using a different encryption passphrase from the one you entered."
                />
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-4">
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
import { VForm, VRow, VCol, VTextField, VCardItem, VDivider, VCardTitle, VBtn, VCard, VCardActions, VDialog, VAlert } from 'vuetify/components';

import { RequiredRule } from '@poc/types/common';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useAppStore } from '@/store/modules/appStore';
import { Bucket } from '@/types/buckets';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const NUMBER_OF_DISPLAYED_OBJECTS = 1000;

const passphrase = ref<string>('');
const isPassphraseVisible = ref<boolean>(false);
const isWarningState = ref<boolean>(false);
const innerContent = ref<Component | null>(null);
const formValid = ref<boolean>(false);

const props = defineProps<{
  modelValue: boolean,
}>();

const emit = defineEmits<{
  (event: 'update:modelValue', value: boolean): void,
  (event: 'passphraseEntered'): void,
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

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

    return data?.objectCount || 0;
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
        if (bucketObjectCount.value > count && bucketObjectCount.value <= NUMBER_OF_DISPLAYED_OBJECTS) {
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
        return;
    }
});
</script>
