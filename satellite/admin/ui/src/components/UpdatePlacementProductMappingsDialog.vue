// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        transition="fade-transition"
        width="800"
    >
        <v-card
            rounded="xlg"
            :title="step === Steps.Form ? 'Update placement-product mappings' : 'Audit'"
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
                    <template v-for="[placement, productId] of mappings" :key="placement">
                        <v-row align="center">
                            <v-col cols="12" md="6">
                                <v-select
                                    label="Placement"
                                    variant="solo-filled"
                                    density="comfortable"
                                    hide-details="auto"
                                    item-title="location"
                                    item-value="id"
                                    :model-value="placement"
                                    :items="availablePlacements"
                                    :rules="[RequiredRule]"
                                    readonly
                                    flat
                                />
                            </v-col>
                            <v-col cols="12" md="5">
                                <v-select
                                    label="Product"
                                    variant="solo-filled"
                                    density="comfortable"
                                    hide-details="auto"
                                    item-title="productName"
                                    item-value="productID"
                                    :model-value="productId"
                                    :items="availableProducts"
                                    :rules="[RequiredRule]"
                                    flat
                                    @update:model-value="(val) => updateMapping(placement, val)"
                                />
                            </v-col>
                            <v-col cols="12" md="1">
                                <v-btn
                                    flat
                                    size="small"
                                    density="comfortable"
                                    :icon="X" rounded="xl"
                                    color="error"
                                    @click="removeMapping(placement)"
                                />
                            </v-col>
                        </v-row>
                        <v-divider v-if="smAndDown" class="my-3" />
                    </template>

                    <v-row class="flex-wrap" align="center">
                        <v-col cols="12" md="6">
                            <v-select
                                v-model="newPlacement"
                                label="Placement"
                                variant="solo-filled"
                                density="comfortable"
                                hide-details="auto"
                                item-title="location"
                                item-value="id"
                                :items="availablePlacementsForNew"
                                :rules="[RequiredRule]"
                                flat
                            />
                        </v-col>
                        <v-col cols="12" md="5">
                            <v-select
                                v-model="newProduct"
                                label="Product"
                                variant="solo-filled"
                                density="comfortable"
                                hide-details="auto"
                                item-title="productName"
                                item-value="productID"
                                :items="availableProducts"
                                :rules="[RequiredRule]"
                                flat
                            />
                        </v-col>
                        <v-col cols="12" md="1">
                            <v-btn
                                flat
                                size="small"
                                density="comfortable"
                                :icon="Plus"
                                rounded="xl"
                                :disabled="newPlacement === null || newProduct === null"
                                @click="addMapping"
                            />
                        </v-col>
                    </v-row>
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
import { Plus, X } from 'lucide-vue-next';
import {
    VBtn,
    VCard,
    VCardActions,
    VCol,
    VSelect,
    VDialog,
    VForm,
    VRow,
    VTextarea,
    VWindow,
    VWindowItem,
    VDivider,
} from 'vuetify/components';
import { useDisplay } from 'vuetify';

import { PlacementInfo, ProductInfo, Project, UpdateProjectEntitlementsRequest } from '@/api/client.gen';
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

const { smAndDown } = useDisplay();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    project: Project;
}>();

const step = ref<Steps>(Steps.Form);
const reason = ref('');
const newPlacement = ref<number | null>(null);
const newProduct = ref<number | null>(null);
const mappings = ref<Map<number, number>>(new Map()); // placementID -> productID

const availablePlacements = computed<PlacementInfo[]>(() => {
    return appStore.state.placements.filter(p => !!p.location).map(p => ({
        id: p.id,
        location: `(${p.id}) - ${p.location}`,
    }));
});

const availablePlacementsForNew = computed<PlacementInfo[]>(() => {
    // Filter out placements that are already in the mappings
    return availablePlacements.value.filter(p => !mappings.value.has(p.id));
});

const availableProducts = computed<ProductInfo[]>(() => appStore.state.products.map(p => ({
    ...p,
    productName: `(${p.productID}) - ${p.productName}`,
})));

// Create a lookup map for faster access
const placementLookup = computed<Map<string,number>>(() => {
    return new Map(availablePlacements.value.map(p => [p.location, p.id]));
});

const productLookup = computed<Map<string,number>>(() => {
    return new Map(availableProducts.value.map(p => [p.productName, p.productID]));
});

// Convert original entitlements to Map for comparison
const originalMappings = computed<Map<number,number>>(() => {
    const result = new Map<number, number>();
    const entitlements = props.project.entitlements?.placementProductMappings;
    if (!entitlements) return result;

    for (const [placementKey, mapping] of Object.entries(entitlements)) {
        const placementId = placementLookup.value.get(placementKey);
        const productId = productLookup.value.get(mapping.productName);
        if (placementId !== undefined && productId !== undefined) {
            result.set(placementId, productId);
        }
    }
    return result;
});

const hasSelectionChanged = computed<boolean>(() => {
    const current = mappings.value;
    const original = originalMappings.value;

    if (current.size !== original.size) return true;

    for (const [placementId, productId] of current) {
        if (original.get(placementId) !== productId) return true;
    }

    return false;
});

const secondaryActionText = computed<string>(() => (step.value === Steps.Form ? 'Cancel' : 'Back'));
const primaryActionText = computed<string>(() => (step.value === Steps.Form ? 'Continue' : 'Submit'));

const submitDisabled = computed(() => {
    if (step.value === Steps.Form) {
        return !hasSelectionChanged.value || mappings.value.size === 0;
    }
    return !reason.value;
});

function addMapping() {
    if (newPlacement.value === null || newProduct.value === null) return;

    mappings.value.set(newPlacement.value, newProduct.value);
    newPlacement.value = null;
    newProduct.value = null;
}

function removeMapping(placementId: number) {
    mappings.value.delete(placementId);
}

function updateMapping(placementId: number, productId: number) {
    mappings.value.set(placementId, productId);
}

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
    if (!reason.value || !hasSelectionChanged.value || mappings.value.size === 0) return;
    withLoading(async () => {
        try {
            const request = new UpdateProjectEntitlementsRequest();
            request.reason = reason.value;
            request.placementProductMappings = Object.fromEntries(mappings.value);

            const updatedEntitlements = await projectsStore.updateEntitlements(props.project.id, request);
            if (projectsStore.state.currentProject) {
                projectsStore.state.currentProject.entitlements = updatedEntitlements;
            }

            notify.success('Placement-product mappings updated successfully.');
            model.value = false;
        } catch (error) {
            notify.error(`Failed to update placement-product mappings. ${error.message}`);
        }
    });
}

function resetForm() {
    step.value = Steps.Form;
    reason.value = '';
    newPlacement.value = null;
    newProduct.value = null;
    mappings.value = new Map(originalMappings.value);
}

watch(model, (newValue) => {
    if (newValue) resetForm();
});
</script>
