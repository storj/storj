// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        transition="fade-transition"
        width="500"
    >
        <v-card
            rounded="xlg"
            :title="step === Steps.Form ? 'Update new buckets placement' : 'Audit'"
            :subtitle="step === Steps.Form ? '' : 'Enter a reason for this change'"
        >
            <template #append>
                <v-btn
                    :icon="X" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <v-window v-model="step" :touch="false" class="pa-6">
                <v-window-item :value="Steps.Form">
                    <v-chip-group v-model="newBucketPlacements" column multiple>
                        <v-chip v-for="placement in availablePlacements" :key="placement.id" :value="placement.location">
                            {{ placement.location }}
                        </v-chip>
                    </v-chip-group>
                </v-window-item>
                <v-window-item :value="Steps.Reason">
                    <v-form :model-value="!!reason" :disabled="isLoading" @submit.prevent="update">
                        <button type="submit" hidden />
                        <v-textarea
                            v-model="reason"
                            :rules="[RequiredRule]"
                            label="Reason"
                            placeholder="Enter reason for this change"
                            hide-details="auto"
                            variant="solo-filled"
                            autofocus
                            flat
                        />
                    </v-form>
                </v-window-item>
            </v-window>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            @click="onSecondaryAction"
                        >
                            {{ secondaryActionText }}
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            :disabled="submitDisabled"
                            :loading="isLoading"
                            block
                            @click="onPrimaryAction"
                        >
                            {{ primaryActionText }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { X } from 'lucide-vue-next';
import {
    VBtn,
    VCard,
    VCardActions,
    VChip,
    VChipGroup,
    VCol,
    VDialog,
    VForm,
    VRow,
    VTextarea,
    VWindow,
    VWindowItem,
} from 'vuetify/components';

import { PlacementInfo, Project, UpdateProjectEntitlementsRequest } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { RequiredRule } from '@/types/common';
import { useProjectsStore } from '@/store/projects';
import { useAppStore } from '@/store/app';

enum Steps {
    Form = 1,
    Reason = 2,
}

const appStore = useAppStore();
const projectsStore = useProjectsStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    project: Project;
}>();

const step = ref<Steps>(Steps.Form);
const reason = ref('');
const newBucketPlacements = ref<string[]>([]);

const availablePlacements = computed<PlacementInfo[]>(() => {
    return appStore.state.placements.filter(p => !!p.location).map(p => ({
        id: p.id,
        location: `(${p.id}) - ${p.location}`,
    }));
});

// Create lookup map for efficient conversion
const placementLookup = computed(() => {
    return new Map(availablePlacements.value.map(p => [p.location, p.id]));
});

const originalPlacements = computed(() => {
    return props.project.entitlements?.newBucketPlacements ?? [];
});

const secondaryActionText = computed(() => (step.value === Steps.Form ? 'Cancel' : 'Back'));
const primaryActionText = computed(() => (step.value === Steps.Form ? 'Continue' : 'Submit'));

const hasSelectionChanged = computed(() => {
    const current = newBucketPlacements.value;
    const original = originalPlacements.value;

    if (current.length !== original.length) return true;

    const originalSet = new Set(original);
    return !current.every(item => originalSet.has(item));
});

const submitDisabled = computed(() => {
    if (step.value === Steps.Form) {
        return !hasSelectionChanged.value || newBucketPlacements.value.length === 0;
    }
    return !reason.value;
});

function onPrimaryAction() {
    if (submitDisabled.value) return;
    if (step.value === Steps.Form) {
        step.value = Steps.Reason;
    } else {
        update();
    }
}

function onSecondaryAction() {
    if (step.value === Steps.Form) {
        model.value = false;
    } else {
        step.value = Steps.Form;
    }
}

function update() {
    if (!reason.value || !hasSelectionChanged.value || newBucketPlacements.value.length === 0) return;
    withLoading(async () => {
        try {
            const request = new UpdateProjectEntitlementsRequest();
            request.reason = reason.value;

            request.newBucketPlacements = newBucketPlacements.value
                .map(location => placementLookup.value.get(location))
                .filter((id): id is number => id !== undefined);

            const updatedEntitlements = await projectsStore.updateEntitlements(props.project.id, request);
            if (projectsStore.state.currentProject) {
                projectsStore.state.currentProject.entitlements = updatedEntitlements;
            }

            notify.success('New buckets placements updated successfully.');
            model.value = false;
        } catch (error) {
            notify.error(`Failed to update new buckets placements. ${error.message}`);
        }
    });
}

function resetForm() {
    step.value = Steps.Form;
    reason.value = '';
    newBucketPlacements.value = [...originalPlacements.value];
}

watch(model, (newValue) => {
    if (newValue) resetForm();
});
</script>
