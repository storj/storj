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
import { useDate } from 'vuetify';

import { ProductInfo } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useAppStore } from '@/store/app';
import { useUsersStore } from '@/store/users';
import { RequiredRule } from '@/types/common';
import { FieldType, FormConfig } from '@/types/forms';
import { licenseTypeOptions } from '@/utils/licenses';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const notify = useNotify();
const usersStore = useUsersStore();
const appStore = useAppStore();
const { isLoading, withLoading } = useLoading();
const date = useDate();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    userId: string;
}>();

const emit = defineEmits<{
    success: [];
}>();

// Product ID 0 means a free license, which has no associated product. Products
// without a license fee are omitted because a license on one is never invoiced,
// and the backend rejects it. The seat price is part of the label so that the
// operator sees what the grant will be billed at before making it.
const productOptions = computed<{ label: string, value: number }[]>(() => [
    { label: 'Free (no product)', value: 0 },
    ...appStore.state.products
        .filter((p: ProductInfo) => Number(p.licenseFeeCents) > 0)
        .map((p: ProductInfo) => ({
            label: `(${p.productID}) - ${p.productName} - $${(Number(p.licenseFeeCents) / 100).toFixed(2)}/seat/month`,
            value: p.productID,
        })),
]);

const initialFormData = computed(() => ({
    type: 'OM',
    productId: 0,
    count: 1,
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
                    type: FieldType.Select,
                    label: 'License Type',
                    items: licenseTypeOptions,
                    itemTitle: 'label',
                    itemValue: 'value',
                    rules: [RequiredRule],
                    required: true,
                }],
            },
            {
                fields: [{
                    key: 'productId',
                    type: FieldType.Select,
                    label: 'Product',
                    items: productOptions.value,
                    itemTitle: 'label',
                    itemValue: 'value',
                    messages: () => ['Licenses on a product with a license fee are billed per seat each month.'],
                }],
            },
            {
                fields: [
                    {
                        key: 'count',
                        type: FieldType.Number,
                        label: 'Seats',
                        min: 1,
                        step: 1,
                    },
                ],
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
            const count = data.count as number;
            await usersStore.grantUserLicense(props.userId, {
                type: data.type as string,
                productId: (data.productId as number) || undefined,
                count: count || undefined,
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
