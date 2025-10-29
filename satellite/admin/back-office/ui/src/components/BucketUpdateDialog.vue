// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Update Project"
        width="550"
        @submit="update"
    />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';

import { useAppStore } from '@/store/app';
import { useBucketsStore } from '@/store/buckets';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { BucketFlags, BucketInfo, BucketState, Project, UpdateBucketRequest } from '@/api/client.gen';
import { FieldType, FormConfig, FormField } from '@/types/forms';
import { RequiredRule } from '@/types/common';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const appStore = useAppStore();
const bucketsStore = useBucketsStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    project: Project;
    bucket: BucketInfo;
}>();

const emit = defineEmits<{
    (e: 'updated', bucket: BucketInfo): void;
}>();

const bucketState = ref<BucketState>();

const featureFlags = computed<BucketFlags>(() => appStore.state.settings.admin.features.bucket);

const placements = computed(() => appStore.state.placements.filter(p => !!p.location));

const initialFormData = computed(() => ({
    userAgent: props.bucket.userAgent ?? '',
    placement: props.bucket.placement ?? 0,
}));

const formConfig = computed((): FormConfig => {
    const config: FormConfig = {
        sections: [{ rows: [] }],
    };

    const firstRowFields: FormField[] = [];
    if (featureFlags.value.updateValueAttribution)
        firstRowFields.push({
            key: 'userAgent',
            type: FieldType.Text,
            label: 'Useragent',
            clearable: true,
            transform: {
                back: (value) => value ?? '',
            },
        });
    if (firstRowFields.length > 0) config.sections[0].rows.push({ fields: firstRowFields });

    const secondRowFields: FormField[] = [];
    if (featureFlags.value.updatePlacement)
        secondRowFields.push({
            key: 'placement',
            type: FieldType.Select,
            label: 'Bucket Placement',
            placeholder: 'Select bucket placement',
            items: placements.value,
            disabled: !bucketState.value?.empty,
            messages: (_) => {
                if (!bucketState.value?.empty) {
                    return ['Placement can only be changed for empty buckets.'];
                }
                return [];
            },
            itemTitle: 'location',
            itemValue: 'id',
            rules: [RequiredRule],
            required: true,
        });

    if (secondRowFields.length > 0) config.sections[0].rows.push({ fields: secondRowFields });

    return config;
});

function update(formData: Record<string, unknown>) {
    withLoading(async () => {
        const request = new UpdateBucketRequest();
        for (const key in request) {
            if (!Object.hasOwn(formData, key)) continue;
            if (formData[key] === initialFormData.value[key]) continue;
            // set only changed fields
            request[key] = formData[key];
        }

        try {
            await bucketsStore.updateBucket(props.project.id, props.bucket.name, request);

            const updated = { ...props.bucket };
            if (request.userAgent !== undefined)
                updated.userAgent = request.userAgent as string;
            if (request.placement !== undefined) {
                const placement = placements.value.find(p => p.id === request.placement);
                updated.placement = placement ? placement.location : 'Unknown';
            }
            model.value = false;
            emit('updated', updated);
            notify.success('Bucket updated successfully!');
        } catch (e) {
            notify.error(`Failed to update bucket. ${e.message}`);
        }
    });
}

watch(() => props.bucket, () => {
    withLoading(async () => {
        try {
            bucketState.value = await bucketsStore.getBucketState(props.project.id, props.bucket.name);
        } catch (error) {
            notify.error('Failed to load bucket state', error);
        }
    });
}, { immediate: true });
</script>