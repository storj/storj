// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="410px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <template #prepend>
                    <img class="d-block" src="@/../static/images/modals/limit.svg" alt="Speedometer">
                </template>
                <v-card-title class="font-weight-bold">Edit {{ limitType }} Limit</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-form v-model="formValid" class="pa-7" @submit.prevent>
                <v-row>
                    <v-col cols="6">
                        <p class="text-subtitle-2 mb-2">Set {{ limitType }} Limit</p>
                        <v-text-field
                            class="edit-project-limit__text-field"
                            variant="outlined"
                            density="comfortable"
                            type="number"
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
                                            :append-icon="isActive ? 'mdi-menu-up' : 'mdi-menu-down'"
                                            v-bind="slotProps"
                                            @mousedown.stop
                                            @click.stop
                                        >
                                            <span class="font-weight-regular">{{ activeMeasurement }}</span>
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
                    <v-col cols="6">
                        <p class="text-subtitle-2 mb-2">Available {{ limitType }}</p>
                        <v-text-field
                            class="edit-project-limit__text-field"
                            variant="solo-filled"
                            density="comfortable"
                            flat
                            readonly
                            :model-value="availableUsageFormatted"
                        >
                            <template #append-inner>
                                <v-menu>
                                    <template #activator="{ props: slotProps, isActive }">
                                        <v-btn
                                            class="h-100 text-medium-emphasis"
                                            variant="text"
                                            density="comfortable"
                                            color="default"
                                            :append-icon="isActive ? 'mdi-menu-up' : 'mdi-menu-down'"
                                            v-bind="slotProps"
                                            @mousedown.stop
                                            @click.stop
                                        >
                                            <span class="font-weight-regular">{{ activeMeasurement }}</span>
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

                    <v-col cols="12">
                        <v-card class="pa-3 mt-n4" variant="flat">
                            <div class="d-flex mx-2 text-subtitle-2 font-weight-bold text-medium-emphasis">
                                0 {{ activeMeasurement }}
                                <v-spacer />
                                {{ availableUsageFormatted }} {{ activeMeasurement }}
                            </div>
                            <v-slider
                                min="0"
                                :max="availableUsage"
                                :step="Memory[activeMeasurement]"
                                color="primary"
                                track-color="default"
                                :model-value="input"
                                @update:model-value="updateInput"
                                hide-details
                            />
                        </v-card>
                    </v-col>
                </v-row>
            </v-form>

            <v-divider />

            <v-card-actions class="pa-7">
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
    VSpacer,
    VSlider,
    VMenu,
    VList,
    VListItem,
} from 'vuetify/components';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';
import { RequiredRule, ValidationRule } from '@poc/types/common';
import { LimitToChange, ProjectLimits } from '@/types/projects';
import { Dimensions, Memory } from '@/utils/bytesSize';
import { decimalShift } from '@/utils/strings';

const projectsStore = useProjectsStore();
const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    modelValue: boolean,
    limitType: LimitToChange,
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean],
}>();

const formValid = ref<boolean>(false);
const activeMeasurement = ref<Dimensions.GB | Dimensions.TB>(Dimensions.TB);
const inputText = ref<string>('0');
const input = ref<number>(0);

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const dropdownModel = computed<(Dimensions.GB | Dimensions.TB)[]>({
    get: () => [ activeMeasurement.value ],
    set: value => activeMeasurement.value = value[0],
});

/**
 * Returns the maximum amount of bytes that the usage limit can be set to.
 */
const availableUsage = computed<number>(() => {
    if (props.limitType === LimitToChange.Storage) {
        return Math.max(
            projectsStore.state.currentLimits.storageLimit,
            parseConfigLimit(configStore.state.config.defaultPaidStorageLimit),
        );
    }
    return Math.max(
        projectsStore.state.currentLimits.bandwidthLimit,
        parseConfigLimit(configStore.state.config.defaultPaidBandwidthLimit),
    );
});

/**
 * Returns the maximum amount of active measurement units that the usage limit can be set to.
 */
const availableUsageFormatted = computed<string>(() => {
    return decimalShift((availableUsage.value / Memory[activeMeasurement.value]).toLocaleString(undefined, { maximumFractionDigits: 2 }), 0);
});

/**
 * Returns an array of validation rules applied to the text input.
 */
const rules = computed<ValidationRule<string>[]>(() => {
    const max = availableUsage.value;
    return [
        RequiredRule,
        v => !(isNaN(+v) || isNaN(parseFloat(v))) || 'Invalid number',
        v => (parseFloat(v) > 0) || 'Number must be positive',
        v => (parseFloat(v) <= max) || 'Number is too large',
    ];
});

/**
 * Parses limit value from config, returning it as a byte amount.
 */
function parseConfigLimit(limit: string): number {
    const [value, unit] = limit.split(' ');
    return parseFloat(value) * Memory[unit === 'B' ? 'Bytes' : unit];
}

/**
 * Updates project limit.
 */
async function onSaveClick(): Promise<void> {
    if (!formValid.value) return;
    await withLoading(async () => {
        try {
            if (props.limitType === LimitToChange.Storage) {
                await projectsStore.updateProjectStorageLimit(new ProjectLimits(0, 0, input.value));
            } else {
                await projectsStore.updateProjectBandwidthLimit(new ProjectLimits(input.value));
            }
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.EDIT_PROJECT_LIMIT);
            return;
        }

        analyticsStore.eventTriggered(
            props.limitType === LimitToChange.Storage
                ? AnalyticsEvent.PROJECT_STORAGE_LIMIT_UPDATED
                : AnalyticsEvent.PROJECT_BANDWIDTH_LIMIT_UPDATED,
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
    const project = projectsStore.state.selectedProject;
    updateInput(
        props.limitType === LimitToChange.Storage
            ? projectsStore.state.currentLimits.storageLimit
            : projectsStore.state.currentLimits.bandwidthLimit,
    );
}, { immediate: true });

watch(() => activeMeasurement.value, unit => {
    inputText.value = (input.value / Memory[unit]).toString();
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
