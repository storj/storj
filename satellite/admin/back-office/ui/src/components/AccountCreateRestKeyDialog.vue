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
                            v-model="expiration" filter column
                            mandatory
                        >
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
                                :min="new Date()"
                                header="Choose Dates"
                                show-adjacent-months
                                border
                                elevation="0"
                                rounded="lg"
                                class="w-100 mb-2"
                            />
                        </v-chip-group>
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
                            block :loading="isLoading"
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
    VBtn,
    VCard,
    VCardActions,
    VChip,
    VChipGroup,
    VCol,
    VDatePicker,
    VDialog,
    VDivider,
    VRow,
    VTextField,
    VAlert,
} from 'vuetify/components';
import {  X } from 'lucide-vue-next';
import { ref, watch } from 'vue';

import { UserAccount } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/users';
import { Duration } from '@/utils/time';

import TextOutputArea from '@/components/TextOutputArea.vue';

const usersStore = useUsersStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const expiration = ref<Duration>(Duration.DAY_30);
const hasCustomDate = ref(false);
const customDate = ref<Date>();
const apiKey = ref('');

function toggleCustomExpiration(hasCustom: boolean = false): void {
    hasCustomDate.value = hasCustom;
    if (!hasCustom) {
        expiration.value = Duration.DAY_30;
        return;
    }
    const today = new Date();
    // set to next day
    today.setDate(today.getDate() + 1);
    customDate.value = today;
}

function createRestKey() {
    withLoading(async () => {
        try {
            const expirationDate = new Date();
            expirationDate.setTime(expirationDate.getTime() + expiration.value.milliseconds);
            apiKey.value = await usersStore.createRestKey(props.account.id, expirationDate);

            notify.success('REST API key created successfully');
        } catch (e) {
            notify.error(`Failed to create REST API key: ${e.message}`);
        }
    });
}

watch(customDate, (newDate) => {
    if (!newDate) return;
    const dur = newDate.getTime() - new Date().getTime();
    expiration.value = new Duration(dur);
});

watch(model, value => {
    if (value) return;
    hasCustomDate.value = false;
    expiration.value = Duration.DAY_30;
    customDate.value = undefined;

    // clear api key after dialog close animation
    setTimeout(() => apiKey.value = '', 300);
});
</script>