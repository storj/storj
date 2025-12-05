// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="400px"
        min-width="400px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="FileKey" :size="18" />
                        </v-sheet>
                    </template>

                    <v-card-title class="font-weight-bold">
                        Add SSH Key
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
            </v-sheet>

            <v-divider />

            <v-form v-model="formValid" class="pa-6" @submit.prevent="submit">
                <p class="mb-6 font-weight-bold">Add an SSH key for VM access:</p>

                <v-textarea
                    v-model="publicKey"
                    label="Public SSH Key"
                    :rules="[RequiredRule, PublicSSHKeyRule]"
                    hint="Paste your public SSH key (usually starts with 'ssh-rsa', 'ssh-dss' or 'ssh-ed25519')"
                    persistent-hint
                    variant="outlined"
                    auto-grow
                    rows="3"
                    class="mb-4"
                    @update:model-value="val => publicKey = val.trim()"
                />

                <v-text-field
                    v-model="name"
                    label="Key name"
                    :rules="[RequiredRule]"
                    hint="A name to identify this key"
                    persistent-hint
                    variant="outlined"
                    class="mb-4"
                    :maxlength="100"
                    @update:model-value="val => name = val.trim()"
                />
            </v-form>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-btn
                    variant="outlined"
                    color="default"
                    class="me-3"
                    min-width="100"
                    block
                    @click="model = false"
                >
                    Cancel
                </v-btn>
                <v-btn
                    variant="flat"
                    color="primary"
                    :loading="isLoading"
                    :disabled="!formValid"
                    min-width="100"
                    block
                    @click="submit"
                >
                    Add Key
                </v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import {
    VDialog,
    VCard,
    VCardTitle,
    VCardActions,
    VCardItem,
    VBtn,
    VTextField,
    VTextarea,
    VForm,
    VDivider,
    VSheet,
} from 'vuetify/components';
import { FileKey, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useComputeStore } from '@/store/modules/computeStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { RequiredRule, PublicSSHKeyRule } from '@/types/common';

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const computeStore = useComputeStore();

const model = defineModel<boolean>({ required: true });

const name = ref<string>('');
const publicKey = ref<string>('');
const formValid = ref<boolean>(false);

function submit(): void {
    if (!formValid.value) return;

    withLoading(async () => {
        try {
            await computeStore.createSSHKey({
                name: name.value,
                publicKey: publicKey.value,
            });

            notify.success('SSH Key added successfully');
            model.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ADD_COMPUTE_SSH_KEY_MODAL);
        }
    });
}

watch(model, (newVal) => {
    if (!newVal) {
        name.value = '';
        publicKey.value = '';
        formValid.value = false;
    }
});
</script>
