// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Update compute access token"
        width="500"
        @submit="update"
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { Project, UpdateProjectEntitlementsRequest } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { FieldType, FormConfig } from '@/types/forms';
import { useProjectsStore } from '@/store/projects';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const projectsStore = useProjectsStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    project: Project;
}>();

const initialFormData = computed(() => ({ token: props.project.entitlements?.computeAccessToken ?? '' }));

const formConfig = computed((): FormConfig => {
    return {
        sections: [
            {
                rows: [
                    {
                        fields: [
                            {
                                key: 'token',
                                type: FieldType.Text,
                                label: 'Compute Access Token',
                                placeholder: 'Compute Access Token',
                                clearable: true,
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
            const request = new UpdateProjectEntitlementsRequest();
            request.computeAccessToken = formData.token as string;
            request.reason = formData.reason as string;
            const entitlements = await projectsStore.updateEntitlements(props.project.id, request);
            const project = { ...props.project, entitlements };
            await projectsStore.updateCurrentProject(project);

            notify.success('Compute access token updated successfully.');
            model.value = false;
        } catch (error) {
            notify.error(`Failed to update compute Access Token. ${error.message}`);
            return;
        }
    });
}
</script>