// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="700" transition="fade-transition">
        <v-card rounded="xlg">
            <template #title>
                Project Limits
            </template>
            <template #subtitle>
                Enter limits for this project
            </template>
            <template #append>
                <v-btn
                    :icon="X" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <v-form v-model="valid" :disableed="isLoading" @submit.prevent="updateLimits">
                <div class="pa-6">
                    <DynamicFormBuilder
                        ref="formBuilder"
                        :config="formConfig"
                        :initial-data="initialFormData"
                    />
                </div>

                <v-card-actions class="pa-6">
                    <v-row>
                        <v-col>
                            <v-btn
                                variant="outlined" color="default"
                                :disabled="isLoading"
                                block @click="model = false"
                            >
                                Cancel
                            </v-btn>
                        </v-col>
                        <v-col>
                            <v-btn
                                color="primary" variant="flat"
                                :loading="isLoading"
                                :disabled="!valid || !hasFormChanged"
                                type="submit"
                                block
                                @click="updateLimits"
                            >
                                Save
                            </v-btn>
                        </v-col>
                    </v-row>
                </v-card-actions>
            </v-form>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VBtn, VCard, VCardActions, VCol, VDialog, VForm, VRow } from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { useNotificationsStore } from '@/store/notifications';
import { Project, ProjectLimitsUpdateRequest, UserProject } from '@/api/client.gen';
import { useProjectsStore } from '@/store/projects';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/users';
import {
    FieldType,
    FormBuilderExpose,
    FormConfig,
    NULLABLE_FIELD_VALUE,
    nullableNumberField,
    rawNumberField,
    terabyteFormField,
} from '@/types/forms';

import DynamicFormBuilder from '@/components/form-builder/DynamicFormBuilder.vue';

const projectsStore = useProjectsStore();
const usersStore = useUsersStore();

const notify = useNotificationsStore();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    project: Project;
}>();

const valid = ref<boolean>(false);
const formBuilder = ref<FormBuilderExpose>();

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

const hasFormChanged = computed(() => {
    const formData = formBuilder.value?.getData() as Record<string, unknown> | undefined;
    if (!formData) return false;

    for (const key in initialFormData.value) {
        if (formData[key] !== initialFormData.value[key]) {
            return true;
        }
    }
    return false;
});

function updateLimits() {
    if (!valid.value) {
        return;
    }
    withLoading(async () => {
        try {
            const request = new ProjectLimitsUpdateRequest();
            const formData = formBuilder.value?.getData() || {};
            if (!formData) return;

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

watch(model, (shown) => {
    if (!shown) return;
    formBuilder.value?.reset();
});
</script>
