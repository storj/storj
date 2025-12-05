// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Update Project"
        width="600"
        @submit="update"
    />
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';

import { Project, UpdateProjectRequest, UserAccount, UserProject } from '@/api/client.gen';
import { useUsersStore } from '@/store/users';
import { useAppStore } from '@/store/app';
import { useProjectsStore } from '@/store/projects';
import { FieldType, FormConfig, FormField } from '@/types/forms';
import { RequiredRule } from '@/types/common';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { ProjectStatus } from '@/types/project';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const appStore = useAppStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    project: Project;
}>();

const placements = computed(() => appStore.state.placements.filter(p => !!p.location));
const projectStatuses = computed(() => projectsStore.state.projectStatuses);
const featureFlags = computed(() => appStore.state.settings.admin.features.project);

const initialFormData = computed(() => ({
    name: props.project.name ?? '',
    description: props.project.description ?? '',
    userAgent: props.project.userAgent ?? '',
    status: props.project.status?.value,
    defaultPlacement: props.project.defaultPlacement ?? 0,
}));

const formConfig = computed((): FormConfig => {
    const config: FormConfig = {
        sections: [{ rows: [] }],
    };

    const firstRowFields: FormField[] = [];
    if (featureFlags.value.updateInfo) firstRowFields.push({
        key: 'name',
        type: FieldType.Text,
        label: 'Project Name',
        rules: [RequiredRule],
    });
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
    if (featureFlags.value.updateInfo)
        secondRowFields.push({
            key: 'status',
            type: FieldType.Select,
            label: 'Project Status',
            placeholder: 'Select project status',
            items: props.project.status?.value === ProjectStatus.PendingDeletion ?
                projectStatuses.value :
                projectStatuses.value.filter(s => s.value !== ProjectStatus.PendingDeletion),
            itemTitle: 'name',
            itemValue: 'value',
            rules: [RequiredRule],
            required: true,
        });
    if (featureFlags.value.updatePlacement)
        secondRowFields.push({
            key: 'defaultPlacement',
            type: FieldType.Select,
            label: 'Project Placement',
            placeholder: 'Select project placement',
            items: placements.value,
            itemTitle: 'location',
            itemValue: 'id',
            rules: [RequiredRule],
            required: true,
        });

    if (secondRowFields.length > 0) config.sections[0].rows.push({ fields: secondRowFields });

    const thirdRowFields: FormField[] = [];
    if (featureFlags.value.updateInfo)
        thirdRowFields.push({
            key: 'description',
            type: FieldType.TextArea,
            label: 'Project Description',
            rules: [RequiredRule],
        });
    if (thirdRowFields.length > 0) config.sections[0].rows.push({ fields: thirdRowFields });

    return config;
});

function update(formData: Record<string, unknown>) {
    withLoading(async () => {
        const request = new UpdateProjectRequest();
        for (const key in request) {
            if (!Object.hasOwn(formData, key)) continue;
            if (formData[key] === initialFormData.value[key]) continue;
            // set only changed fields
            request[key] = formData[key];
        }

        try {
            const project = await projectsStore.updateProject(props.project.id, request);

            model.value = false;
            notify.success('Project updated successfully!');

            if (projectsStore.state.currentProject?.id === project.id) {
                await projectsStore.updateCurrentProject(project);
            }

            const account = usersStore.state.currentAccount as UserAccount;
            if (!account || !account.projects) return;

            const index = account.projects.findIndex((p) => p.id === project.id);
            if (index === -1) return;
            const accountProject = account.projects[index] as UserProject;
            accountProject.name = project.name;
            accountProject.active = project.status?.value === ProjectStatus.Active;

            await usersStore.updateCurrentUser(account);
        } catch (e) {
            notify.error(`Failed to update project. ${e.message}`);
        }
    });
}

onMounted(() => {
    withLoading(async () => {
        try {
            await projectsStore.getProjectStatuses();
        } catch (e) {
            notify.error(e);
        }
    });
});
</script>