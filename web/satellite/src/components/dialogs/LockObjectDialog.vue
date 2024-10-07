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
        <v-card ref="innerContent" rounded="xlg">
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
                        Lock
                    </v-card-title>
                    <template #append>
                        <v-btn
                            icon="$close"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-row>
                <v-col class="pa-6 mx-3">
                    <p class="my-2">
                        Enabling object lock will prevent objects from being deleted or overwritten for a specified period of time.
                    </p>

                    <p class="mb-2 text-body-2">
                        Select default object lock mode (Optional):
                    </p>

                    <v-chip-group
                        v-model="lockType"
                        class="mb-4"
                        selected-class="text-primary font-weight-bold"
                        mandatory
                        column
                        filter
                    >
                        <v-chip v-for="type in [GOVERNANCE_LOCK, COMPLIANCE_LOCK]" :key="type" variant="outlined":value="type">
                            {{ type.substring(0, 1) + type.substring(1).toLowerCase() }}
                        </v-chip>
                    </v-chip-group>

                    <v-alert v-if="lockType === COMPLIANCE_LOCK" variant="tonal" color="default">
                        <p class="font-weight-bold text-body-2 mb-1">Enable Object Lock (Compliance Mode)</p>
                        <p class="text-subtitle-2">No user, including the project owner can overwrite, delete, or alter object lock settings.</p>
                    </v-alert>
                    <v-alert v-if="lockType === GOVERNANCE_LOCK" variant="tonal" color="default">
                        <p class="font-weight-bold text-body-2 mb-1">Enable Object Lock (Governance Mode)</p>
                        <p class="text-subtitle-2">Authorized users with special permissions can bypass retention settings and delete or modify objects.</p>
                    </v-alert>

                    <p class="my-2 font-weight-bold text-body-2">
                        Select the lock retention period:
                    </p>

                    <v-select
                        v-model="selectedRange"
                        clearable
                        variant="outlined"
                        :items="ranges"
                        item-title="label"
                        return-object
                    />

                    <v-date-picker
                        v-if="selectedRange?.label == customRangeLabel.label"
                        v-model="customUntilDate"
                        width="100%"
                        header="Choose Date"
                        show-adjacent-months
                        border
                        elevation="0"
                        rounded="lg"
                    />
                </v-col>
            </v-row>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
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
                            :disabled="!selectedRange?.date && !customUntilDate"
                            :loading="isLoading"
                            block
                            @click="lockObject"
                        >
                            Set Lock
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import {
    VBtn,
    VAlert,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VChip,
    VChipGroup,
    VCol, VDatePicker,
    VDialog,
    VDivider,
    VRow,
    VSheet,
    VSelect,
} from 'vuetify/components';
import { Lock } from 'lucide-vue-next';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { Time } from '@/utils/time';
import { COMPLIANCE_LOCK, GOVERNANCE_LOCK, ObjLockMode, LockUntilRange } from '@/types/objectLock';
import { dateAfterDays } from '@/utils/date';

const customRangeLabel = { label: 'Choose a custom date' };
const ranges: LockUntilRange[] = [
    { label: '1 Day', date: dateAfterDays(1) },
    { label: '1 Week', date: dateAfterDays(7) },
    { label: '2 Weeks', date: dateAfterDays(14) },
    { label: '1 Month', date: dateAfterDays(30) },
    { label: '6 Months', date: dateAfterDays(180) },
    { label: '1 Year', date: dateAfterDays(365) },
    { label: '3 Years', date: dateAfterDays(1095) },
    { label: '5 Years', date: dateAfterDays(1825) },
    { label: '7 Years', date: dateAfterDays(2555) },
    { label: '10 Years', date: dateAfterDays(3650) },
    customRangeLabel,
];

const obStore = useObjectBrowserStore();

const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const model = defineModel<boolean>({ required: false });
const props = defineProps<{
    file: BrowserObject | null
}>();

const emit = defineEmits<{
    'contentRemoved': [],
    'fileLocked': [],
}>();

const innerContent = ref<VCard | null>(null);

const selectedRange = ref<LockUntilRange>();
const lockType = ref<ObjLockMode>();

const customUntilDate = ref<Date>();

function lockObject() {
    withLoading(async () => {
        if (!props.file) {
            return;
        }
        if (!lockType.value) {
            notify.warning('Select a lock type');
            return;
        }
        let date: Date | undefined;
        if (selectedRange.value?.label === customRangeLabel.label) {
            date = customUntilDate.value;
        } else {
            date = selectedRange.value?.date;
        }
        if (!date) {
            notify.warning('Select a date');
            return;
        }
        try {
            await obStore.lockObject(props.file, lockType.value, date);
            notify.success(`Object locked until ${Time.formattedDate(date)}`);
            emit('fileLocked');
            model.value = false;
        } catch (e) {
            notify.notifyError(e, AnalyticsErrorEventSource.LOCK_OBJECT_DIALOG);
            return;
        }
    });
}

watch(selectedRange, (_) => {
    console.log(selectedRange);

    customUntilDate.value = undefined;
});

watch(innerContent, comp => {
    if (comp) {
        return;
    }
    emit('contentRemoved');
    lockType.value = undefined;
    selectedRange.value = undefined;
});
</script>
