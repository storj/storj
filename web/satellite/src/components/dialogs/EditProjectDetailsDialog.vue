// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="iconComponent" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Project {{ field }}</v-card-title>
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

            <v-divider />

            <v-form v-model="formValid" class="pa-6" @submit.prevent>
                <v-text-field
                    v-model="input"
                    class="pt-4"
                    variant="outlined"
                    :rules="rules"
                    :label="`Project ${field}`"
                    :counter="maxLength"
                    :maxlength="maxLength"
                    persistent-counter
                    :hide-details="false"
                    autofocus
                />
            </v-form>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block :loading="isLoading" @click="onSaveClick">
                            Save
                        </v-btn>
                    </v-col>
                </v-row>
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
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VForm,
    VTextField,
    VSheet,
} from 'vuetify/components';
import { Pencil, NotebookPen, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/composables/useNotify';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { ValidationRule } from '@/types/common';
import { FieldToChange, ProjectFields, MAX_NAME_LENGTH, MAX_DESCRIPTION_LENGTH } from '@/types/projects';

const props = defineProps<{
    field: FieldToChange,
}>();

const model = defineModel<boolean>({ required: true });

const projectsStore = useProjectsStore();
const analyticsStore = useAnalyticsStore();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const formValid = ref<boolean>(false);
const input = ref<string>('');

/**
 * Returns the maximum input length.
 */
const maxLength = computed<number>(() => {
    return props.field === FieldToChange.Name ? MAX_NAME_LENGTH : MAX_DESCRIPTION_LENGTH;
});

/**
 * Returns an array of validation rules applied to the input.
 */
const rules = computed<ValidationRule<string>[]>(() => {
    const max = maxLength.value;
    const required = props.field === FieldToChange.Name;
    return [
        v => (!!v || !required) || 'Required',
        v => v.length <= max || 'Input is too long.',
    ];
});

/**
 * Updates project field.
 */
async function onSaveClick(): Promise<void> {
    if (!formValid.value) return;
    await withLoading(async () => {
        try {
            if (props.field === FieldToChange.Name) {
                await projectsStore.updateProjectName(new ProjectFields(input.value, ''));
            } else {
                await projectsStore.updateProjectDescription(new ProjectFields('', input.value));
            }
        } catch (error) {
            error.message = `Error updating project ${props.field.toLowerCase()}. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.EDIT_PROJECT_DETAILS);
            return;
        }

        analyticsStore.eventTriggered(
            props.field === FieldToChange.Name
                ? AnalyticsEvent.PROJECT_NAME_UPDATED
                : AnalyticsEvent.PROJECT_DESCRIPTION_UPDATED,
            { project_id: projectsStore.state.selectedProject.id },
        );
        notify.success(`Project ${props.field.toLowerCase()} updated.`);

        model.value = false;
    });
}

watch(() => model.value, shown => {
    if (!shown) return;
    const project = projectsStore.state.selectedProject;
    input.value = props.field === FieldToChange.Name ? project.name : project.description;
}, { immediate: true });

const iconComponent = computed(() => {
    if (props.field === FieldToChange.Name) {
        return Pencil;
    } else {
        return NotebookPen;
    }
});
</script>
