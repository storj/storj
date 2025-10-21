// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card ref="innerContent">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="Key" :size="18" />
                        </v-sheet>
                    </template>

                    <v-card-title class="font-weight-bold">
                        New API Key
                    </v-card-title>

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
            </v-sheet>

            <v-divider />

            <v-card-text class="pa-0">
                <v-window v-model="step" :touch="false">
                    <v-window-item :value="Step.Create">
                        <v-form v-model="formValid" class="pa-6" @submit.prevent="createAPIKey">
                            <v-row>
                                <v-col cols="12">
                                    <v-text-field
                                        v-model="name"
                                        label="Access Name"
                                        placeholder="Enter a name for this access key"
                                        variant="outlined"
                                        autofocus
                                        :hide-details="false"
                                        :rules="[RequiredRule, MaxNameLengthRule]"
                                        maxlength="100"
                                        class="mb-n5 mt-2"
                                        required
                                    />
                                </v-col>

                                <v-col cols="12" class="pb-0">
                                    <label class="mb-2">API Key Duration</label>
                                    <v-chip-group
                                        v-model="expiration" filter column
                                        mandatory
                                    >
                                        <v-chip :value="Duration.ZERO" @click="toggleCustomExpiration()">
                                            No Expiration
                                        </v-chip>

                                        <v-divider class="my-2" />

                                        <v-chip
                                            v-for="dur in [Duration.DAY_30, Duration.DAY_60, Duration.DAY_180, Duration.YEAR_1]"
                                            :key="dur.days"
                                            :value="dur" variant="outlined"
                                            @click="toggleCustomExpiration()"
                                        >
                                            <span class="text-capitalize">{{ dur.shortString }}</span>
                                        </v-chip>

                                        <v-divider class="my-2" />

                                        <v-chip-group v-model="hasCustomDate" filter>
                                            <v-chip :value="true" @click="toggleCustomExpiration(!hasCustomDate)">
                                                Set Custom Expiration Date
                                            </v-chip>
                                        </v-chip-group>

                                        <v-date-picker
                                            v-if="hasCustomDate"
                                            v-model="customDate"
                                            :allowed-dates="allowDate"
                                            header="Choose Dates"
                                            show-adjacent-months
                                            border
                                            elevation="0"
                                            rounded="lg"
                                            class="w-100"
                                        />
                                    </v-chip-group>
                                </v-col>

                                <v-col cols="12" class="pt-0">
                                    <v-alert>
                                        This API key will have administrative access to manage your projects.
                                        Store it securely and never share it publicly.
                                    </v-alert>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>
                    <v-window-item :value="Step.Success">
                        <div class="pa-6">
                            <v-alert class="mb-5" variant="tonal" color="success">
                                <p class="font-weight-bold">API Key Generated Successfully</p>
                                Make sure to copy your API key now. For Security reasons,
                                you won't be able to see it again after closing this window.
                            </v-alert>

                            <text-output-area
                                label="API Key"
                                :value="apiKey"
                                show-copy
                            />
                        </div>
                    </v-window-item>
                </v-window>
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            @click="model = false"
                        >
                            {{ step === Step.Create ? 'Cancel' : 'Close' }}
                        </v-btn>
                    </v-col>
                    <v-col v-if="step === Step.Create">
                        <v-btn
                            color="primary"
                            variant="flat"
                            :disabled="!formValid"
                            :loading="isLoading"
                            block
                            @click="createAPIKey"
                        >
                            Generate API Key
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
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardText,
    VCardTitle,
    VChip,
    VChipGroup,
    VCol,
    VDatePicker,
    VDialog,
    VDivider,
    VForm,
    VRow,
    VSheet,
    VTextField,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import { Key, X } from 'lucide-vue-next';

import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { Duration } from '@/utils/time';
import { useRestApiKeysStore } from '@/store/modules/apiKeysStore';
import { MaxNameLengthRule, RequiredRule } from '@/types/common';

import TextOutputArea from '@/components/dialogs/accessSetupSteps/TextOutputArea.vue';

enum Step {
    Create,
    Success,
}

const apiKeysStore = useRestApiKeysStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const formValid = ref(false);
const name = ref('');
const expiration = ref<Duration>(Duration.ZERO);
const hasCustomDate = ref(false);
const customDate = ref<Date>();
const step = ref(Step.Create);
const apiKey = ref('');

const innerContent = ref<VCard | null>(null);

function allowDate(date: unknown): boolean {
    if (!date) return false;
    const d = new Date(date as string);
    if (isNaN(d.getTime())) return false;

    d.setHours(0, 0, 0, 0);
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    return d > today;
}

function createAPIKey(): void {
    withLoading(async () => {
        if (!formValid.value) return;
        try {
            apiKey.value = await apiKeysStore.createAPIKey(name.value, expiration.value.nanoseconds);
            step.value = Step.Success;
            await apiKeysStore.getKeys();
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.CREATE_API_KEY_DIALOG);
        }
    });
}

function toggleCustomExpiration(hasCustom: boolean = false): void {
    hasCustomDate.value = hasCustom;
    if (!hasCustom) {
        expiration.value = Duration.ZERO;
        return;
    }
    const today = new Date();
    // set to next day
    today.setDate(today.getDate() + 1);
    customDate.value = today;
}

watch(customDate, (newDate) => {
    if (!newDate) return;
    const dur = newDate.getTime() - new Date().getTime();
    expiration.value = new Duration(dur);
});

watch(innerContent, comp => {
    if (!comp) {
        name.value = '';
        hasCustomDate.value = false;
        expiration.value = Duration.ZERO;
        step.value = Step.Create;
        customDate.value = undefined;
        return;
    }
});
</script>

<style lang="scss" scoped>
:deep(.v-slide-group__content){
    justify-content: space-between;
}
</style>