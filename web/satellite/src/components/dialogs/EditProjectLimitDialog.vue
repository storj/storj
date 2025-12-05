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
                <v-card-title class="font-weight-bold">{{ hasCustomLimit ? 'Edit' : 'Set' }} {{ limitType }} Limit</v-card-title>
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
                <v-row>
                    <v-col cols="12" sm="6">
                        <p class="text-subtitle-2 mb-2">Current Limit</p>
                        <v-text-field
                            class="edit-project-limit__text-field"
                            variant="solo-filled"
                            density="comfortable"
                            flat
                            readonly
                            :model-value="currentLimitFormatted"
                        >
                            <template #append-inner>
                                <v-menu>
                                    <template #activator="{ props: slotProps, isActive }">
                                        <v-btn
                                            class="h-100 text-medium-emphasis"
                                            variant="text"
                                            density="comfortable"
                                            color="default"
                                            :append-icon="isActive ? ChevronUp : ChevronDown"
                                            v-bind="slotProps"
                                            @mousedown.stop
                                            @click.stop
                                        >
                                            <span class="font-weight-regular">{{ currentLimitFormatted !== NO_LIMIT ? activeMeasurement : '' }}</span>
                                        </v-btn>
                                    </template>
                                    <v-list v-model:selected="dropdownModel" density="compact">
                                        <v-list-item :title="Dimensions.TB" :value="Dimensions.TB" />
                                        <v-list-item :title="Dimensions.GB" :value="Dimensions.GB" />
                                    </v-list>
                                </v-menu>
                            </template>
                        </v-text-field>
                    </v-col>
                    <v-col cols="12" sm="6">
                        <p class="text-subtitle-2 mb-2">Set {{ limitType }} Limit</p>
                        <v-text-field
                            class="edit-project-limit__text-field"
                            variant="outlined"
                            density="comfortable"
                            :type="inputText !== NO_LIMIT ? 'number' : undefined"
                            :rules="rules"
                            :hide-details="false"
                            :model-value="inputText"
                            maxlength="50"
                            @update:model-value="updateInputText"
                        >
                            <template #append-inner>
                                <v-menu>
                                    <template #activator="{ props: slotProps, isActive }">
                                        <v-btn
                                            class="h-100 text-medium-emphasis"
                                            variant="text"
                                            density="comfortable"
                                            color="default"
                                            :append-icon="isActive ? ChevronUp : ChevronDown"
                                            v-bind="slotProps"
                                            @mousedown.stop
                                            @click.stop
                                        >
                                            <span class="font-weight-regular">{{ inputText !== NO_LIMIT ? activeMeasurement : '' }}</span>
                                        </v-btn>
                                    </template>
                                    <v-list v-model:selected="dropdownModel" density="compact">
                                        <v-list-item :title="Dimensions.TB" :value="Dimensions.TB" />
                                        <v-list-item :title="Dimensions.GB" :value="Dimensions.GB" />
                                    </v-list>
                                </v-menu>
                            </template>
                        </v-text-field>
                    </v-col>

                    <v-col v-if="hasCustomLimit" cols="12">
                        <v-card class="pa-2 pl-4 mt-n4" variant="flat">
                            <div class="d-flex justify-space-between align-center">
                                <div><p class="text-body-2">Don't need a limit?</p></div>
                                <div>
                                    <v-btn :loading="isLoading" variant="text" @click="unSetLimit">
                                        Remove Limit
                                    </v-btn>
                                </div>
                            </div>
                        </v-card>
                    </v-col>
                </v-row>

                <v-alert
                    class="mt-3"
                    density="compact"
                    type="info"
                    variant="tonal"
                    text="Limit updates may take several minutes to be reflected."
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
                        <v-btn color="primary" variant="flat" block :disabled="!hasChanged || !formValid" :loading="isLoading" @click="onSaveClick">
                            {{ shouldContactSupport ? "Contact support" : "Save" }}
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
    VMenu,
    VList,
    VListItem,
    VAlert,
    VSheet,
} from 'vuetify/components';
import { ChevronDown, ChevronUp, Cloud, CloudDownload, X } from 'lucide-vue-next';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { RequiredRule, ValidationRule } from '@/types/common';
import { LimitToChange } from '@/types/projects';
import { Dimensions, Memory } from '@/utils/bytesSize';
import { decimalShift } from '@/utils/strings';
import { useConfigStore } from '@/store/modules/configStore';

const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const analyticsStore = useAnalyticsStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    limitType: LimitToChange,
}>();

const model = defineModel<boolean>({ required: true });

const NO_LIMIT = 'No Limit';
const formValid = ref<boolean>(false);
const activeMeasurement = ref<Dimensions.GB | Dimensions.TB>(Dimensions.TB);
const inputText = ref<string>('0');
const input = ref<number>(0);

const dropdownModel = computed<(Dimensions.GB | Dimensions.TB)[]>({
    get: () => [ activeMeasurement.value ],
    set: value => {
        if (value[0]) activeMeasurement.value = value[0];
    },
});

/**
 * Whether the new no-limits UI is enabled.
 */
const noLimitsUiEnabled = computed((): boolean => {
    return configStore.state.config.noLimitsUiEnabled;
});

const hasCustomLimit = computed(() => {
    if (props.limitType === LimitToChange.Storage) {
        return !!currentLimits.value.userSetStorageLimit;
    }
    return !!currentLimits.value.userSetBandwidthLimit;
});

/**
 * Returns the current project limits from store.
 */
const currentLimits = computed(() => projectsStore.state.currentLimits);

/**
 * Returns whether the limit has been changed.
 */
const hasChanged = computed(() => {
    let limit = currentLimits.value.userSetBandwidthLimit;
    if (props.limitType === LimitToChange.Storage) {
        limit = currentLimits.value.userSetStorageLimit;
    }
    return limit !== input.value;
});

/**
 * If the user's preferred limit is greater than their project limit,
 * they should contact support.
 */
const shouldContactSupport = computed<boolean>(() => {
    const projLimit = props.limitType === LimitToChange.Storage
        ? currentLimits.value.storageLimit
        : currentLimits.value.bandwidthLimit;
    return input.value > projLimit;
});

/**
 * Returns the maximum amount of active measurement units that the usage limit can be set to.
 */
const currentLimitFormatted = computed<string>(() => {
    const customLimit = props.limitType === LimitToChange.Storage
        ? currentLimits.value.userSetStorageLimit
        : currentLimits.value.userSetBandwidthLimit;
    const limit = props.limitType === LimitToChange.Storage
        ? currentLimits.value.storageLimit
        : currentLimits.value.bandwidthLimit;
    if (noLimitsUiEnabled.value && !customLimit) {
        return NO_LIMIT;
    }
    return decimalShift(((customLimit || limit) / Memory[activeMeasurement.value]).toLocaleString(undefined, { maximumFractionDigits: 2 }), 0);
});

/**
 * Returns an array of validation rules applied to the text input.
 */
const rules = computed<ValidationRule<string>[]>(() => {
    return [
        RequiredRule,
        v => v === NO_LIMIT || !(isNaN(+v) || isNaN(parseFloat(v))) || 'Invalid number',
        v => v === NO_LIMIT || (parseFloat(v) > 0) || 'Number must be positive',
    ];
});

/**
 * Resets the limit to the default value.
 */
function unSetLimit(): void {
    const limit = props.limitType === LimitToChange.Storage
        ? currentLimits.value.storageLimit
        : currentLimits.value.bandwidthLimit;
    inputText.value = noLimitsUiEnabled.value ? NO_LIMIT : (limit / Memory[activeMeasurement.value]).toString();
    input.value = 0;
}

/**
 * Updates project limit.
 */
async function onSaveClick(): Promise<void> {
    if (shouldContactSupport.value) {
        window.open(configStore.state.config.projectLimitsIncreaseRequestURL, '_blank', 'noreferrer');
        return;
    }
    if (!formValid.value) return;
    await withLoading(async () => {
        try {
            if (props.limitType === LimitToChange.Storage) {
                await projectsStore.updateProjectStorageLimit(input.value);
            } else {
                await projectsStore.updateProjectBandwidthLimit(input.value);
            }
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.EDIT_PROJECT_LIMIT);
            return;
        }

        analyticsStore.eventTriggered(
            props.limitType === LimitToChange.Storage
                ? AnalyticsEvent.PROJECT_STORAGE_LIMIT_UPDATED
                : AnalyticsEvent.PROJECT_BANDWIDTH_LIMIT_UPDATED,
            { project_id: projectsStore.state.selectedProject.id },
        );
        notify.success('Limit updated successfully.');

        model.value = false;
    });
}

/**
 * Updates input refs with value from text field.
 */
function updateInputText(value: string): void {
    inputText.value = value;
    const num = +value;
    if (isNaN(num) || isNaN(parseFloat(value))) return;
    input.value = Math.floor(num * Memory[activeMeasurement.value]);
}

/**
 * Updates input refs with value from slider.
 */
function updateInput(value: number): void {
    input.value = value;
    inputText.value = (value / Memory[activeMeasurement.value]).toString();
}

watch(() => model.value, shown => {
    if (!shown) return;
    updateInput(
        props.limitType === LimitToChange.Storage
            ? (currentLimits.value.userSetStorageLimit || currentLimits.value.storageLimit)
            : (currentLimits.value.userSetBandwidthLimit || currentLimits.value.bandwidthLimit),
    );
}, { immediate: true });

watch(() => activeMeasurement.value, unit => {
    inputText.value = (input.value / Memory[unit]).toString();
});

const iconComponent = computed(() => {
    return props.limitType === LimitToChange.Storage ? Cloud : CloudDownload;
});
</script>

<style scoped lang="scss">
.edit-project-limit__text-field {

    :deep(.v-field) {
        padding-inline-end: 0;
    }

    :deep(input) {
        text-overflow: ellipsis;

        /* Firefox */
        appearance: textfield;

        /* Chrome, Safari, Edge, Opera */

        &::-webkit-outer-spin-button,
        &::-webkit-inner-spin-button {
            appearance: none;
            margin: 0;
        }
    }
}
</style>
