// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Grant License"
        subtitle="Grant a new license to this user"
        width="600"
        @submit="grantLicense"
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useDate } from 'vuetify/framework';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/users';
import { RequiredRule } from '@/types/common';
import { FieldType, FormConfig } from '@/types/forms';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const notify = useNotify();
const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
const date = useDate();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    userId: string;
}>();

const emit = defineEmits<{
    success: [];
}>();

const initialFormData = computed(() => ({
    type: '',
    publicId: '',
    bucketName: '',
    expiresAt: null as Date | null,
    key: '',
}));

const formConfig = computed((): FormConfig => ({
    sections: [{
        rows: [
            {
                fields: [{
                    key: 'type',
                    type: FieldType.Text,
                    label: 'License Type',
                    placeholder: 'e.g., object-mount',
                    rules: [RequiredRule],
                    required: true,
                }],
            },
            {
                fields: [
                    {
                        key: 'publicId',
                        type: FieldType.Text,
                        label: 'Public ID (Optional)',
                        placeholder: 'Leave empty for all projects',
                        errorMessages: (_value, formData) => {
                            const data = formData as Record<string, unknown> | undefined;
                            if (data?.bucketName && !data?.publicId) {
                                return 'Public ID is required when bucket name is set';
                            }
                            return undefined;
                        },
                    },
                    {
                        key: 'bucketName',
                        type: FieldType.Text,
                        label: 'Bucket Name (Optional)',
                        placeholder: 'Leave empty for all buckets',
                    },
                ],
            },
            {
                fields: [{
                    key: 'key',
                    type: FieldType.Text,
                    label: 'Key (Optional)',
                    placeholder: 'Leave empty if not needed',
                }],
            },
            {
                fields: [{
                    key: 'expiresAt',
                    type: FieldType.Date,
                    label: 'Expiration Date',
                    rules: [RequiredRule],
                    required: true,
                    prependIcon: '',
                    min: date.addDays(new Date(), 1) as Date,
                    transform: {
                        forward: (value) => value ? date.date(value) : null,
                        back: (value) => value ? (date.date(value) as Date).toISOString() : '',
                    },
                }],
            },
        ],
    }],
}));

async function grantLicense(data: Record<string, unknown>) {
    await withLoading(async () => {
        try {
            await usersStore.grantUserLicense(props.userId, {
                type: data.type as string,
                publicId: (data.publicId as string) || undefined,
                bucketName: (data.bucketName as string) || undefined,
                expiresAt: data.expiresAt as string,
                key: (data.key as string) || undefined,
                reason: data.reason as string,
            });
            notify.success('License granted successfully');
            model.value = false;
            emit('success');
        } catch (error) {
            notify.error('Failed to grant license', error);
        }
    });
}
</script>
