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
import { PositiveNumberRule, RequiredRule } from '@/types/common';
import { Project, ProjectLimitsUpdateRequest, UserProject } from '@/api/client.gen';
import { useProjectsStore } from '@/store/projects';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/users';
import { FieldType, FormBuilderExpose, FormConfig, FormField, rawNumberField, terabyteFormField } from '@/types/forms';

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
    maxBuckets: props.project?.maxBuckets ?? 0,
    storageLimit: props.project?.storageLimit ?? 0,
    userSetStorageLimit: props.project?.userSetStorageLimit ?? 0,
    bandwidthLimit: props.project?.bandwidthLimit ?? 0,
    userSetBandwidthLimit: props.project?.userSetBandwidthLimit ?? 0,
    segmentLimit: props.project?.segmentLimit ?? 0,
    rateLimit: props.project?.rateLimit ?? 0,
    burstLimit: props.project?.burstLimit ?? 0,
    rateLimitList: props.project?.rateLimitList ?? 0,
    burstLimitList: props.project?.burstLimitList ?? 0,
    rateLimitGet: props.project?.rateLimitGet ?? 0,
    burstLimitGet: props.project?.burstLimitGet ?? 0,
    rateLimitPut: props.project?.rateLimitPut ?? 0,
    burstLimitPut: props.project?.burstLimitPut ?? 0,
    rateLimitDelete: props.project?.rateLimitDelete ?? 0,
    burstLimitDelete: props.project?.burstLimitDelete ?? 0,
    rateLimitHead: props.project?.rateLimitHead ?? 0,
    burstLimitHead: props.project?.burstLimitHead ?? 0,
    projectId: props.project?.id ?? '',
}));

const formConfig = computed((): FormConfig => {
    return {
        sections: [
            {
                rows: [
                    {
                        fields: [
                            rawNumberField({ key: 'maxBuckets', label: 'Total buckets',
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
                            rateAndBurstFields({ key: 'rateLimit', label: 'Rate' }, false),
                            rateAndBurstFields({ key: 'burstLimit', label: 'Burst' }, false),
                            rateAndBurstFields({ key: 'rateLimitHead', label: 'Head rate' }),
                        ],
                    }, {
                        fields: [
                            rateAndBurstFields({ key: 'burstLimitHead', label: 'Head burst' }),
                            rateAndBurstFields({ key: 'rateLimitGet', label: 'Get rate' }),
                            rateAndBurstFields({ key: 'burstLimitGet', label: 'Get burst' }),
                        ],
                    }, {
                        fields: [
                            rateAndBurstFields({ key: 'rateLimitList', label: 'List rate' }),
                            rateAndBurstFields({ key: 'burstLimitList', label: 'List burst' }),
                            rateAndBurstFields({ key: 'rateLimitPut', label: 'Put rate' }),
                        ],
                    }, {
                        fields: [
                            rateAndBurstFields({ key: 'burstLimitPut', label: 'Put burst' }),
                            rateAndBurstFields({ key: 'rateLimitDelete', label: 'Delete rate' }),
                            rateAndBurstFields({ key: 'burstLimitDelete', label: 'Delete burst' }),
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

function rateAndBurstFields(conf: Partial<FormField>, optional = true): FormField {
    return {
        type: FieldType.Number,
        rules: [RequiredRule, PositiveNumberRule],
        key: conf.key ?? '',
        label: conf.label ?? '',
        clearable: !optional,
        step: 100,
        cols: { default: 12, sm: 4 },
        transform: optional ? {
            forward: (value) => value === null || value === undefined ? 0 : value,
            back: (value) => value === null || value === undefined ? 0 : value,
        } : undefined,
    };
}

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
