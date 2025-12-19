// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
    >
        <v-card rounded="xlg">
            <template #title>
                Create REST API Key
            </template>
            <template v-if="!apiKey" #subtitle>
                <span class="text-wrap">
                    Select the valid duration for the REST API key
                </span>
            </template>
            <template #append>
                <v-btn :icon="X" variant="text" size="small" color="default" @click="model = false" />
            </template>

            <div v-if="!apiKey" class="pa-6">
                <v-row>
                    <v-col cols="12" class="pb-0">
                        <v-chip-group
                            v-model="expirationDate" filter column
                            mandatory
                        >
                            <v-chip
                                v-for="date in dates"
                                :key="date.txt"
                                :value="date.date"
                                variant="outlined"
                                @click="toggleCustomExpiration()"
                            >
                                <span class="text-capitalize">{{ date.txt }}</span>
                            </v-chip>

                            <v-divider class="my-2" />

                            <div style="position: relative;">
                                <v-date-input
                                    ref="dateInput"
                                    v-model="customDate"
                                    style="position: absolute; z-index: -1;"
                                    class="invisible"
                                    label="Set Custom Expiration Date"
                                    prepend-icon=""
                                    variant="solo"
                                />
                                <v-chip-group :model-value="!!customDate" filter>
                                    <v-chip :value="true" @click="toggleCustomExpiration(!customDate)">
                                        <span v-if="customDate">{{ useDate().format(customDate, 'fullDate') }}</span>
                                        <span v-else>Set Custom Expiration Date</span>
                                    </v-chip>
                                </v-chip-group>
                            </div>
                        </v-chip-group>
                    </v-col>

                    <v-col cols="12">
                        <v-textarea
                            v-model="reason"
                            :rules="[RequiredRule]"
                            placeholder="Enter reason for creating this API key"
                            label="Reason"
                            variant="solo-filled"
                            hide-details="auto"
                            autofocus
                            flat
                        />
                    </v-col>
                </v-row>

                <v-row>
                    <v-col cols="12">
                        <v-text-field
                            :model-value="account.email" label="Account Email" variant="solo-filled" flat readonly
                            hide-details="auto"
                        />
                    </v-col>
                </v-row>
            </div>
            <div v-else class="pa-6">
                <v-alert class="mb-5" variant="tonal" color="success">
                    <p class="font-weight-bold">API Key Generated Successfully</p>
                    Make sure to copy your API key now. It will not be shown again.
                </v-alert>

                <TextOutputArea label="API Key" :value="apiKey" />
            </div>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block :disabled="isLoading"
                            @click="model = false"
                        >
                            {{ apiKey ? 'Close' : 'Cancel' }}
                        </v-btn>
                    </v-col>
                    <v-col v-if="!apiKey">
                        <v-btn
                            color="primary" variant="flat"
                            :loading="isLoading"
                            :disabled="!reason || !expirationDate"
                            block
                            @click="createRestKey"
                        >
                            Create
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VChip,
    VChipGroup,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VTextarea,
    VTextField,
} from 'vuetify/components';
import { VDateInput } from 'vuetify/labs/VDateInput';
import { X } from 'lucide-vue-next';
import { computed, ref, watch } from 'vue';
import { useDate } from 'vuetify';

import { UserAccount } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/users';
import { RequiredRule } from '@/types/common';

import TextOutputArea from '@/components/TextOutputArea.vue';

const usersStore = useUsersStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const expirationDate = ref<Date | null>(null);
const customDate = ref<Date | null>(null);
const apiKey = ref('');
const reason = ref('');

const dateInput = ref<InstanceType<typeof VDateInput> | null>(null);

const dates = computed<{ date: Date, txt: string }[]>(() => {
    const today = new Date();
    return [
        {
            date: new Date(today.getFullYear(), today.getMonth(), today.getDate() + 30),
            txt: '30 days',
        },
        {
            date: new Date(today.getFullYear(), today.getMonth(), today.getDate() + 60),
            txt: '60 days',
        },
        {
            date: new Date(today.getFullYear(), today.getMonth(), today.getDate() + 180),
            txt: '180 days',
        },
        {
            date: new Date(today.getFullYear() + 1, today.getMonth(), today.getDate()),
            txt: '1 year',
        },
    ];
});

function toggleCustomExpiration(hasCustom: boolean = false): void {
    if (!hasCustom) {
        expirationDate.value = dates.value[0].date;
        customDate.value = null;
        return;
    }
    const today = new Date();
    // set to next day
    today.setDate(today.getDate() + 1);
    customDate.value = today;
    dateInput.value?.click();
}

function createRestKey() {
    withLoading(async () => {
        if (!expirationDate.value || !reason.value) return;
        try {
            apiKey.value = await usersStore.createRestKey(props.account.id, expirationDate.value, reason.value);

            notify.success('REST API key created successfully');
        } catch (e) {
            notify.error(`Failed to create REST API key: ${e.message}`);
        }
    });
}

watch(customDate, (newDate) => {
    if (!newDate) return;
    expirationDate.value = newDate;
});

watch(model, value => {
    if (value) return;
    expirationDate.value = dates.value[0].date;
    customDate.value = null;
    reason.value = '';

    // clear api key after dialog close animation
    setTimeout(() => apiKey.value = '', 300);
});
</script>