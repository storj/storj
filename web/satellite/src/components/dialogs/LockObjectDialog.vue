// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="400px"
        max-width="450px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card ref="innerContent" :loading="isLoading" rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="Lock" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        {{ title }}
                    </v-card-title>
                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-window v-model="step" :touch="false" class="overflow-y-auto">
                <v-window-item :value="LockStep.Settings">
                    <v-row>
                        <v-col class="pa-6 mx-3">
                            <p class="my-2">
                                {{ info }}
                            </p>

                            <p class="mt-4 mb-2 font-weight-bold text-body-2">
                                Name:
                            </p>

                            <v-chip
                                variant="tonal"
                                filter
                                value="filename"
                                color="default"
                                class="mb-2 font-weight-bold"
                            >
                                {{ file?.Key }}
                            </v-chip>

                            <template v-if="file?.VersionId">
                                <p class="my-2 font-weight-bold text-body-2">
                                    Version:
                                </p>

                                <v-chip
                                    variant="tonal"
                                    filter
                                    color="default"
                                    class="mb-4 font-weight-bold"
                                >
                                    {{ file?.VersionId }}
                                </v-chip>
                            </template>

                            <template v-if="!existingRetention.active">
                                <p class="my-2 font-weight-bold text-body-2">
                                    Select lock type:
                                </p>

                                <p class="mb-2 text-body-2">
                                    Governance allows authorized users to modify the lock.
                                    Compliance prevents any changes to the lock.
                                </p>

                                <v-chip-group
                                    v-model="lockType"
                                    class="mb-4"
                                    selected-class="font-weight-bold"
                                    mandatory
                                    column
                                    filter
                                >
                                    <v-chip v-for="type in [GOVERNANCE_LOCK, COMPLIANCE_LOCK]" :key="type" :value="type" variant="outlined">
                                        {{ type.substring(0, 1) + type.substring(1).toLowerCase() }}
                                    </v-chip>
                                </v-chip-group>
                            </template>

                            <template v-if="existingRetention.active">
                                <p class="mb-2 font-weight-bold text-body-2">
                                    Current lock expiration:
                                </p>

                                <v-chip
                                    variant="tonal"
                                    filter
                                    color="default"
                                    class="mb-4 font-weight-bold"
                                >
                                    {{ getFormattedExpiration(existingRetention.retainUntil) }}
                                </v-chip>
                            </template>

                            <p class="mb-2 font-weight-bold text-body-2">
                                {{ existingRetention.active ? 'Extend lock by:' : 'Select the lock retention period:' }}
                            </p>

                            <v-chip-group
                                v-model="selectedRange"
                                class="mb-4"
                                selected-class="font-weight-bold"
                                mandatory
                                column
                                filter
                            >
                                <v-chip v-for="range in ranges" :key="range.label" :value="range">
                                    {{ range.label }}
                                </v-chip>
                            </v-chip-group>

                            <v-date-picker
                                v-if="selectedRange?.label == customRangeLabel.label"
                                v-model="customUntilDate"
                                :allowed-dates="allowDate"
                                width="100%"
                                header="Choose Date"
                                show-adjacent-months
                                border
                                elevation="0"
                                rounded="lg"
                            />
                        </v-col>
                    </v-row>
                </v-window-item>
                <v-window-item :value="LockStep.Confirmation">
                    <v-row>
                        <v-col class="pa-6 mx-3">
                            <p class="my-2">
                                This file has been locked successfully.
                            </p>

                            <p class="mt-4 mb-2 font-weight-bold text-body-2">
                                Name:
                            </p>

                            <v-chip
                                variant="tonal"
                                filter
                                color="default"
                                class="mb-2 font-weight-bold"
                            >
                                {{ file?.Key }}
                            </v-chip>

                            <template v-if="file?.VersionId">
                                <p class="my-2 font-weight-bold text-body-2">
                                    Version:
                                </p>

                                <v-chip
                                    variant="tonal"
                                    filter
                                    color="default"
                                    class="mb-2 font-weight-bold"
                                >
                                    {{ file?.VersionId }}
                                </v-chip>
                            </template>

                            <template v-if="!!lockedUntil">
                                <p class="my-2 font-weight-bold text-body-2">
                                    Lock expiration:
                                </p>

                                <v-chip
                                    variant="tonal"
                                    filter
                                    color="default"
                                    class="mb-2 font-weight-bold"
                                >
                                    {{ lockedUntil }}
                                </v-chip>
                            </template>
                        </v-col>
                    </v-row>
                </v-window-item>
            </v-window>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col v-if="step === LockStep.Settings">
                        <v-btn
                            variant="outlined"
                            color="default"
                            :disabled="isLoading"
                            block
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            :disabled="nextButtonDisabled"
                            :loading="isLoading"
                            block
                            @click="onLockOrExit"
                        >
                            {{ nextButtonLabel }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VChip,
    VChipGroup,
    VCol,
    VDatePicker,
    VDialog,
    VDivider,
    VRow,
    VSheet,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import { Lock, X } from 'lucide-vue-next';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { Time } from '@/utils/time';
import { COMPLIANCE_LOCK, GOVERNANCE_LOCK, ObjLockMode, Retention } from '@/types/objectLock';

enum LockStep {
    Settings,
    Confirmation,
}

interface LockUntilRange {
    label: string,
    date?: Date,
}

const customRangeLabel = { label: 'Choose a custom date' };

const obStore = useObjectBrowserStore();

const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const model = defineModel<boolean>({ required: false });
const props = defineProps<{
    file: BrowserObject | null
}>();

const emit = defineEmits<{
    'contentRemoved': [],
}>();

const innerContent = ref<VCard | null>(null);

const step = ref<LockStep>(LockStep.Settings);
const selectedRange = ref<LockUntilRange>();
const lockType = ref<ObjLockMode>();
const customUntilDate = ref<Date>();
const existingRetention = ref<Retention>(new Retention('', new Date()));

const ranges = computed<LockUntilRange[]>(() => {
    const initDate = existingRetention.value.active ? existingRetention.value.retainUntil : new Date();

    return [
        { label: '1 Day', date: dateAfterDays(initDate, 1) },
        { label: '1 Week', date: dateAfterDays(initDate, 7) },
        { label: '2 Weeks', date: dateAfterDays(initDate, 14) },
        { label: '1 Month', date: dateAfterDays(initDate, 30) },
        { label: '6 Months', date: dateAfterDays(initDate, 180) },
        { label: '1 Year', date: dateAfterDays(initDate, 365) },
        { label: '3 Years', date: dateAfterDays(initDate, 1095) },
        { label: '5 Years', date: dateAfterDays(initDate, 1825) },
        { label: '7 Years', date: dateAfterDays(initDate, 2555) },
        { label: '10 Years', date: dateAfterDays(initDate, 3650) },
        customRangeLabel,
    ];
});

const title = computed<string>(() => {
    if (existingRetention.value.active) {
        return step.value === LockStep.Settings ? 'Extend Lock' : 'Lock Extended Successfully';
    }

    return step.value === LockStep.Settings ? 'Lock' : 'Lock Successful';
});

const info = computed<string>(() => {
    if (existingRetention.value.active) {
        return 'This file is currently locked and cannot be deleted or overwritten.';
    }

    return 'Locking this version will prevent it from being deleted or overwritten for the specified period of time.';
});

const nextButtonLabel = computed<string>(() => {
    if (existingRetention.value.active) {
        return step.value === LockStep.Settings ? 'Extend Lock' : 'Close';
    }

    return step.value === LockStep.Settings ? 'Set Lock' : 'Close';
});

const nextButtonDisabled = computed<boolean>(() => {
    return (!existingRetention.value.active && !lockType.value) ||
        (!selectedRange.value?.date && !customUntilDate.value);
});

const lockedUntil = computed<string>(() => {
    const until = selectedRange.value?.label === customRangeLabel.label ? getModifiedCustomDate() : selectedRange.value?.date;
    if (!until) {
        return '';
    }

    return getFormattedExpiration(until);
});

function getFormattedExpiration(date: Date): string {
    return `${
        Time.formattedDateWithGMTOffset(date)} at
        ${date.toLocaleTimeString('en-GB', { hour: 'numeric', minute: 'numeric' })}
    `;
}

function allowDate(date: unknown): boolean {
    if (!date) return false;
    const d = new Date(date as string);
    if (isNaN(d.getTime())) return false;

    d.setHours(0, 0, 0, 0);
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    return d >= today;
}

function getModifiedCustomDate(): Date {
    if (!customUntilDate.value) {
        return new Date();
    }

    const date = existingRetention.value ? new Date(existingRetention.value.retainUntil) : new Date();
    date.setDate(customUntilDate.value.getDate());
    date.setMonth(customUntilDate.value.getMonth());
    date.setFullYear(customUntilDate.value.getFullYear());

    return date;
}

function dateAfterDays(initDate: Date, days: number): Date {
    const laterDate = new Date(initDate);
    laterDate.setDate(initDate.getDate() + days);
    return laterDate;
}

function onLockOrExit(): void {
    if (step.value === LockStep.Settings) {
        if (existingRetention.value.active) {
            extendLock();
        } else {
            lockObject();
        }
    } else {
        model.value = false;
    }
}

function lockObject(): void {
    withLoading(async () => {
        if (!props.file) {
            return;
        }
        if (!lockType.value) {
            notify.warning('Select a lock type');
            return;
        }

        const date = selectLockDate();
        if (!date) {
            notify.warning('Select a date');
            return;
        }
        try {
            await obStore.lockObject(props.file, lockType.value, date);
            notify.success(`Object locked until ${Time.formattedDate(date)}`);

            step.value = LockStep.Confirmation;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.LOCK_OBJECT_DIALOG);
        }
    });
}

function extendLock(): void {
    withLoading(async () => {
        if (!props.file) {
            return;
        }
        if (!existingRetention.value.mode) {
            notify.error('Unknown existing retention mode. Please restart the flow', AnalyticsErrorEventSource.LOCK_OBJECT_DIALOG);
            return;
        }

        const date = selectLockDate();
        if (!date) {
            notify.warning('Select a date');
            return;
        }

        try {
            await obStore.lockObject(props.file, existingRetention.value.mode, date);
            notify.success(`Object lock extended until ${Time.formattedDate(date)}`);

            step.value = LockStep.Confirmation;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.LOCK_OBJECT_DIALOG);
        }
    });
}

function selectLockDate(): Date | undefined {
    if (selectedRange.value?.label === customRangeLabel.label) {
        return getModifiedCustomDate();
    } else {
        return selectedRange.value?.date;
    }
}

watch(selectedRange, (_) => {
    customUntilDate.value = undefined;
});

watch(innerContent, comp => {
    if (comp) {
        withLoading(async () => {
            if (!props.file) return;

            const ret = await obStore.getObjectRetention(props.file);
            if (ret.active) existingRetention.value = ret;
        });

        return;
    }
    emit('contentRemoved');
    lockType.value = undefined;
    customUntilDate.value = undefined;
    selectedRange.value = undefined;
    step.value = LockStep.Settings;
    existingRetention.value = new Retention('', new Date());
});
</script>
