// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Update Project Limits"
        subtitle="Enter limits for this project"
        width="600"
        overflow
        @submit="update"
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { useNotificationsStore } from '@/store/notifications';
import { Project, ProjectLimitsUpdateRequest, UserProject } from '@/api/client.gen';
import { useProjectsStore } from '@/store/projects';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/users';
import { FieldType,
    FormConfig,
    NULLABLE_FIELD_VALUE,
    nullableNumberField,
    rawNumberField,
    terabyteFormField,
} from '@/types/forms';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const projectsStore = useProjectsStore();
const usersStore = useUsersStore();

const notify = useNotificationsStore();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    project: Project;
}>();

const initialFormData = computed(() => ({
    maxBuckets: props.project?.maxBuckets ?? NULLABLE_FIELD_VALUE,
    storageLimit: props.project?.storageLimit ?? 0,
    userSetStorageLimit: props.project?.userSetStorageLimit ?? NULLABLE_FIELD_VALUE,
    bandwidthLimit: props.project?.bandwidthLimit ?? 0,
    userSetBandwidthLimit: props.project?.userSetBandwidthLimit ?? NULLABLE_FIELD_VALUE,
    segmentLimit: props.project?.segmentLimit ?? 0,
    rateLimit: props.project?.rateLimit ?? NULLABLE_FIELD_VALUE,
    burstLimit: props.project?.burstLimit ?? NULLABLE_FIELD_VALUE,
    rateLimitList: props.project?.rateLimitList ?? NULLABLE_FIELD_VALUE,
    burstLimitList: props.project?.burstLimitList ?? NULLABLE_FIELD_VALUE,
    rateLimitGet: props.project?.rateLimitGet ?? NULLABLE_FIELD_VALUE,
    burstLimitGet: props.project?.burstLimitGet ?? NULLABLE_FIELD_VALUE,
    rateLimitPut: props.project?.rateLimitPut ?? NULLABLE_FIELD_VALUE,
    burstLimitPut: props.project?.burstLimitPut ?? NULLABLE_FIELD_VALUE,
    rateLimitDelete: props.project?.rateLimitDelete ?? NULLABLE_FIELD_VALUE,
    burstLimitDelete: props.project?.burstLimitDelete ?? NULLABLE_FIELD_VALUE,
    rateLimitHead: props.project?.rateLimitHead ?? NULLABLE_FIELD_VALUE,
    burstLimitHead: props.project?.burstLimitHead ?? NULLABLE_FIELD_VALUE,
    projectId: props.project?.id ?? '',
}));

const formConfig = computed((): FormConfig => {
    return {
        sections: [
            {
                rows: [
                    {
                        fields: [
                            nullableNumberField({
                                key: 'maxBuckets',
                                label: 'Total buckets',
                                step: 1,
                                cols:{ default: 12, sm: 6 },
                            }),
                            rawNumberField({ key: 'segmentLimit', label: 'Segments', step: 5000,
                                cols: { default: 12, sm: 6 },
                            }),
                        ],
                    },
                ],
            }, {
                divider: { text: 'Storage / Bandwidth limits' },
                rows: [
                    {
                        fields: [
                            terabyteFormField({
                                key: 'storageLimit', label: 'Storage (TB)',
                                cols: { default: 12, sm: 6 },
                            }),
                            terabyteFormField({
                                key:'userSetStorageLimit', label:'User set storage (TB)',
                                cols: { default: 12, sm: 6 },
                                clearable: true,
                            }),
                        ],
                    }, {
                        fields: [
                            terabyteFormField({
                                key: 'bandwidthLimit', label: 'Download per month (TB)',
                                cols: { default: 12, sm: 6 },
                            }),
                            terabyteFormField({ key:'userSetBandwidthLimit', label:'User set download per month (TB)',
                                cols: { default: 12, sm: 6 },
                                clearable: true,
                            }),
                        ],
                    },
                ],
            },
            {
                divider: { text: 'Rate / Burst limits' },
                rows: [
                    {
                        fields: [
                            nullableNumberField({ key: 'rateLimit', label: 'Rate' }),
                            nullableNumberField({ key: 'burstLimit', label: 'Burst' }),
                            nullableNumberField({ key: 'rateLimitHead', label: 'Head rate' }),
                        ],
                    }, {
                        fields: [
                            nullableNumberField({ key: 'burstLimitHead', label: 'Head burst' }),
                            nullableNumberField({ key: 'rateLimitGet', label: 'Get rate' }),
                            nullableNumberField({ key: 'burstLimitGet', label: 'Get burst' }),
                        ],
                    }, {
                        fields: [
                            nullableNumberField({ key: 'rateLimitList', label: 'List rate' }),
                            nullableNumberField({ key: 'burstLimitList', label: 'List burst' }),
                            nullableNumberField({ key: 'rateLimitPut', label: 'Put rate' }),
                        ],
                    }, {
                        fields: [
                            nullableNumberField({ key: 'burstLimitPut', label: 'Put burst' }),
                            nullableNumberField({ key: 'rateLimitDelete', label: 'Delete rate' }),
                            nullableNumberField({ key: 'burstLimitDelete', label: 'Delete burst' }),
                        ],
                    },
                ],
            },
            {
                divider: { },
                rows: [
                    {
                        fields: [
                            {
                                key: 'projectId',
                                type: FieldType.Text,
                                label: 'Project ID',
                                readonly: true,
                            },
                        ],
                    },
                ],
            },
        ],
    };
});

function update(formData: Record<string, unknown>) {
    withLoading(async () => {
        try {
            const request = new ProjectLimitsUpdateRequest();
            for (const key in request) {
                if (!Object.hasOwn(formData, key)) continue;
                // set only changed fields
                if (formData[key] === initialFormData.value[key]) continue;
                request[key] = formData[key];
            }

            const project = await projectsStore.updateProjectLimits(props.project.id, request);
            model.value = false;
            notify.notifySuccess('Successfully updated project limits.');

            if (projectsStore.state.currentProject?.id === props.project.id) {
                await projectsStore.updateCurrentProject(project);
            }

            const account = usersStore.state.currentAccount;
            if (!account || !account.projects) return;

            const index = account.projects.findIndex((p) => p.id === project.id);
            if (index === -1) return;
            const accountProject = account.projects[index] as UserProject;
            accountProject.segmentLimit = project.segmentLimit ?? 0;
            accountProject.storageLimit = project.storageLimit ?? 0;
            accountProject.bandwidthLimit = project.bandwidthLimit ?? 0;
            accountProject.userSetBandwidthLimit = project.userSetBandwidthLimit;
            accountProject.userSetStorageLimit = project.userSetStorageLimit;

            await usersStore.updateCurrentUser(account);
        } catch (error) {
            notify.notifyError(`Error updating project limits. ${error.message}`);
        }
    });
}
</script>
