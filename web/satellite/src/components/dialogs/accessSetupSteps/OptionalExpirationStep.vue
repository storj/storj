// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-6" @submit.prevent>
        <v-row>
            <v-col cols="12">
                <p class="font-weight-bold mb-2">
                    Optional Expiration
                </p>
                <p>You can choose if you want the access to expire.</p>

                <v-chip-group
                    v-model="expiration"
                    column
                    mandatory
                    selected-class="text-primary font-weight-bold"
                    class="mt-2"
                >
                    <v-chip
                        :key="Expiration.No"
                        :value="Expiration.No"
                        variant="outlined"
                        filter
                        color="default"
                    >
                        {{ Expiration.No }}
                    </v-chip>

                    <v-divider class="my-2" />

                    <v-chip
                        :key="Expiration.Day"
                        :value="Expiration.Day"
                        variant="outlined"
                        filter
                        color="secondary"
                    >
                        {{ Expiration.Day }}
                    </v-chip>

                    <v-chip
                        :key="Expiration.Week"
                        :value="Expiration.Week"
                        variant="outlined"
                        filter
                        color="green"
                    >
                        {{ Expiration.Week }}
                    </v-chip>

                    <v-chip
                        :key="Expiration.Month"
                        :value="Expiration.Month"
                        variant="outlined"
                        filter
                        color="help"
                    >
                        {{ Expiration.Month }}
                    </v-chip>

                    <v-chip
                        :key="Expiration.Year"
                        :value="Expiration.Year"
                        variant="outlined"
                        filter
                        color="warning"
                    >
                        {{ Expiration.Year }}
                    </v-chip>

                    <v-divider class="my-2" />

                    <v-chip
                        :key="Expiration.Custom"
                        :value="Expiration.Custom"
                        variant="outlined"
                        filter
                        color="warning"
                        @click="isDatePicker = true"
                    >
                        {{ Expiration.Custom }}
                    </v-chip>
                </v-chip-group>
                <v-alert class="mt-4" variant="tonal" color="info" width="auto">
                    <p class="text-subtitle-2">{{ endDate ? endDate.toLocaleString() : 'No end date' }}</p>
                </v-alert>
            </v-col>
        </v-row>

        <v-overlay v-model="isDatePicker" persistent class="align-center justify-center">
            <v-date-picker
                v-model="datePickerModel"
                @update:model-value="onDatePickerSubmit"
            >
                <template #header />
            </v-date-picker>
        </v-overlay>
    </v-form>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import {
    VAlert,
    VChip,
    VChipGroup,
    VCol,
    VForm,
    VRow,
    VDivider,
    VOverlay,
    VDatePicker,
} from 'vuetify/components';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';

enum Expiration {
    No = 'No Expiration',
    Day = '1 Day',
    Week = '1 Week',
    Month = '1 Month',
    Year = '1 Year',
    Custom = 'Set Custom Expiration Date',
}

defineProps<{
    endDate: Date | null
}>();

const emit = defineEmits<{
    'endDateChanged': [date: Date | null];
}>();

const notify = useNotify();

const expiration = ref<Expiration>(Expiration.No);
const isDatePicker = ref<boolean>(false);
const datePickerModel = ref<Date>();

/**
 * Returns the current date offset by the specified amount.
 */
function getNowOffset(days = 0, months = 0, years = 0): Date {
    const now = new Date();
    return new Date(
        now.getFullYear() + years,
        now.getMonth() + months,
        now.getDate() + days,
        11, 59, 59,
    );
}

/**
 * Stores the access grant end date from the date picker.
 */
function onDatePickerSubmit(): void {
    if (!datePickerModel.value) return;

    const date = datePickerModel.value;
    const submitted = new Date(
        date.getFullYear(),
        date.getMonth(),
        date.getDate(),
        11, 59, 59,
    );

    if (submitted.getTime() < new Date().getTime()) {
        notify.error('Please select future date', AnalyticsErrorEventSource.SETUP_ACCESS_MODAL);
        return;
    }

    emit('endDateChanged', submitted);

    isDatePicker.value = false;
}

watch(expiration, value => {
    switch (value) {
    case Expiration.No:
        emit('endDateChanged', null);
        break;
    case Expiration.Day:
        emit('endDateChanged', getNowOffset(1));
        break;
    case Expiration.Week:
        emit('endDateChanged', getNowOffset(7));
        break;
    case Expiration.Month:
        emit('endDateChanged', getNowOffset(0, 1));
        break;
    case Expiration.Year:
        emit('endDateChanged', getNowOffset(0, 0, 1));
    }
});
</script>
