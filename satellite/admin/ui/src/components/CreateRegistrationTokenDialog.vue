// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-if="!createdToken"
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Create Registration Token"
        subtitle="Create a token that allows a user to register"
        width="500"
        @submit="onSubmit"
    />

    <v-dialog v-else v-model="model" transition="fade-transition" width="500">
        <v-card rounded="xlg">
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-card-title class="font-weight-bold">
                        Registration Token Created
                    </v-card-title>
                </template>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false;"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <div class="pa-6">
                <v-alert type="success" variant="tonal" class="mb-4">
                    Token created successfully! Share this token or registration link with the user.
                </v-alert>
                <TextOutputArea label="Registration Token" :value="createdToken" class="mb-4" />
                <TextOutputArea label="Registration Link" :value="registrationLink" class="mb-4" />
                <p v-if="tokenExpiresAt" class="text-body-2 text-medium-emphasis">
                    This token expires on {{ new Date(tokenExpiresAt).toLocaleString() }}.
                </p>
            </div>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-btn
                    color="primary"
                    variant="flat"
                    block
                    @click="model = false;"
                >
                    Done
                </v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VCardActions,
    VBtn,
    VDivider,
    VAlert,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { useUsersStore } from '@/store/users';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { FieldType, FormConfig } from '@/types/forms';
import { PositiveNumberOrEmptyRule, PositiveNumberRule, RequiredRule } from '@/types/common';
import { useAppStore } from '@/store/app';
import { CreateRegistrationTokenRequest } from '@/api/client.gen';
import { Memory } from '@/utils/memory';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';
import TextOutputArea from '@/components/TextOutputArea.vue';

const usersStore = useUsersStore();
const appStore = useAppStore();

const model = defineModel<boolean>({ required: true });

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const createdToken = ref<string | null>(null);
const tokenExpiresAt = ref<string | null | undefined>(null);

const expirationOptions = [
    { label: '1 day', value: '24h' },
    { label: '3 days', value: '72h' },
    { label: '7 days', value: '168h' },
    { label: '14 days', value: '336h' },
    { label: '30 days', value: '720h' },
    { label: 'No expiration', value: '' },
];

const initialFormData = computed(() => ({
    projectLimit: null,
    storageLimit: null,
    bandwidthLimit: null,
    segmentLimit: null,
    expiresIn: '',
    reason: '',
}));

const formConfig = computed((): FormConfig => {
    return {
        sections: [
            {
                rows: [
                    {
                        fields: [
                            {
                                key: 'projectLimit',
                                type: FieldType.Number,
                                label: 'Project Limit',
                                min: 1,
                                step: 1,
                                rules: [RequiredRule, PositiveNumberRule],
                                required: true,
                            },

                        ],
                    },
                    {
                        fields: [
                            {
                                key: 'storageLimit',
                                type: FieldType.Number,
                                label: 'Storage Limit (TB)',
                                min: 0.1,
                                step: 0.1,
                                precision: 1,
                                rules: [PositiveNumberOrEmptyRule],
                                required: false,
                            },
                        ],
                    },
                    {
                        fields: [
                            {
                                key: 'bandwidthLimit',
                                type: FieldType.Number,
                                label: 'Bandwidth Limit (TB)',
                                min: 0.1,
                                step: 0.1,
                                precision: 1,
                                rules: [PositiveNumberOrEmptyRule],
                                required: false,
                            },
                        ],
                    },
                    {
                        fields: [
                            {
                                key: 'segmentLimit',
                                type: FieldType.Number,
                                label: 'Segment Limit',
                                min: 1,
                                step: 1,
                                rules: [PositiveNumberOrEmptyRule],
                                required: false,
                            },
                        ],
                    },
                    {
                        fields: [
                            {
                                key: 'expiresIn',
                                type: FieldType.Select,
                                label: 'Expiration',
                                items: expirationOptions,
                                itemTitle: 'label',
                                itemValue: 'value',
                                required: false,
                            },
                        ],
                    },
                ],
            },
        ],
    };
});

const registrationLink = computed<string>(() => {
    if (!createdToken.value) return '';

    const url = new URL('/signup', appStore.state.settings.console.externalAddress);
    url.searchParams.set('token', createdToken.value);

    return url.toString();
});

function onSubmit(formData: Record<string, unknown>): void {
    withLoading(async () => {
        try {
            const request: CreateRegistrationTokenRequest = {
                projectLimit: formData.projectLimit as number,
                reason: formData.reason as string,
            };

            if (formData.storageLimit !== null && formData.storageLimit !== undefined) {
                request.storageLimit = Math.round(formData.storageLimit as number * Memory.TB);
            }
            if (formData.bandwidthLimit !== null && formData.bandwidthLimit !== undefined) {
                request.bandwidthLimit = Math.round(formData.bandwidthLimit as number * Memory.TB);
            }
            if (formData.segmentLimit !== null && formData.segmentLimit !== undefined) {
                request.segmentLimit = formData.segmentLimit as number;
            }
            if (formData.expiresIn) {
                request.expiresIn = formData.expiresIn as string;
            }

            const response = await usersStore.createRegistrationToken(request);
            createdToken.value = response.token;
            tokenExpiresAt.value = response.expiresAt;

            notify.success('Registration token created successfully!');
        } catch (error) {
            notify.error(`Failed to create token: ${error.message}`);
        }
    });
}

watch(model, (newValue) => {
    if (!newValue) {
        setTimeout(() => {
            createdToken.value = null;
            tokenExpiresAt.value = null;
        }, 300);
    }
});
</script>
