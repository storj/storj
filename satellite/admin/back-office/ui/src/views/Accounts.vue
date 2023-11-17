// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container fluid>
        <v-row>
            <v-col cols="6">
                <PageTitleComponent title="Accounts" />
                <PageSubtitleComponent subtitle="Find accounts on North America US1." />
            </v-col>

            <v-col cols="6" class="d-flex justify-end align-center">
                <v-btn variant="outlined" color="default">
                    <svg width="16" height="16" class="mr-2" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path
                            d="M10 1C14.9706 1 19 5.02944 19 10C19 14.9706 14.9706 19 10 19C5.02944 19 1 14.9706 1 10C1 5.02944 5.02944 1 10 1ZM10 2.65C5.94071 2.65 2.65 5.94071 2.65 10C2.65 14.0593 5.94071 17.35 10 17.35C14.0593 17.35 17.35 14.0593 17.35 10C17.35 5.94071 14.0593 2.65 10 2.65ZM10.7496 6.8989L10.7499 6.91218L10.7499 9.223H12.9926C13.4529 9.223 13.8302 9.58799 13.8456 10.048C13.8602 10.4887 13.5148 10.8579 13.0741 10.8726L13.0608 10.8729L10.7499 10.873L10.75 13.171C10.75 13.6266 10.3806 13.996 9.925 13.996C9.48048 13.996 9.11807 13.6444 9.10066 13.2042L9.1 13.171L9.09985 10.873H6.802C6.34637 10.873 5.977 10.5036 5.977 10.048C5.977 9.60348 6.32857 9.24107 6.76882 9.22366L6.802 9.223H9.09985L9.1 6.98036C9.1 6.5201 9.46499 6.14276 9.925 6.12745C10.3657 6.11279 10.7349 6.45818 10.7496 6.8989Z"
                            fill="currentColor"
                        />
                    </svg>
                    New Account
                    <NewAccountDialog />
                </v-btn>
            </v-col>
        </v-row>

        <!-- <v-row class="d-flex align-center justify-center mt-2">
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="All Accounts" data="218,748" color="default"/>
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Enterprise" data="3,405" color="default" />
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Priority" data="5,224" color="default"/>
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Pro" data="82,386" color="default"/>
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Free" data="123,480" color="default"/>
            </v-col>
            <v-col cols="12" sm="6" md="4" lg="2">
                <CardStatsComponent title="Suspended" data="1" color="default" />
            </v-col>
        </v-row> -->

        <v-row align="center" justify="center">
            <v-col cols="12" sm="8" md="6" lg="4">
                <v-card variant="flat" class="mt-8 pa-4" rounded="xlg" border>
                    <v-card-text>
                        <v-form v-model="isFormValid" @submit.prevent="goToUser">
                            <h2 class="my-1">Find an account</h2>
                            <p>Enter account email</p>
                            <v-text-field
                                v-model="email"
                                label="Email"
                                variant="outlined"
                                class="mt-5"
                                :disabled="isLoading"
                                autofocus
                                :rules="emailRules"
                                :error-messages="notFoundError ? 'The user was not found.' : ''"
                                @click="goToUser"
                            />
                            <v-btn class="mt-3" block size="large" :loading="isLoading" @click="goToUser">
                                Continue
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { VContainer, VRow, VCol, VBtn, VCard, VCardText, VForm, VTextField } from 'vuetify/components';

import { useAppStore } from '@/store/app';
import { useNotificationsStore } from '@/store/notifications';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import NewAccountDialog from '@/components/NewAccountDialog.vue';

const isLoading = ref<boolean>(false);
const isFormValid = ref<boolean>(false);
const email = ref<string>('');
const notFoundError = ref<boolean>(false);

const emailRules: ((value: string) => boolean | string)[] = [
    v => /.+@.+\..+/.test(v) || 'E-mail must be valid.',
    v => !!v || 'Required',
];

const appStore = useAppStore();
const notify = useNotificationsStore();
const router = useRouter();

/**
 * Fetches user information and navigates to Account Details page.
 * Displays an error message if no user has the input email address.
 */
async function goToUser(): Promise<void> {
    if (isLoading.value || !isFormValid.value) return;
    isLoading.value = true;

    const maxAttempts = 3;
    const retryDelay = 1000;
    for (let attempt = 0; attempt < maxAttempts; attempt++) {
        try {
            await appStore.getUserByEmail(email.value);
            router.push(`/account-details`);
            isLoading.value = false;
            return;
        } catch (error) {
            if (error.responseStatusCode === 404) {
                notFoundError.value = true;
                break;
            } else if (error.responseStatusCode === 409) {
                if (attempt >= maxAttempts-1) {
                    notify.notifyError(`Error getting user. Please wait a few minutes before trying again.`);
                    break;
                }
                await new Promise(resolve => setTimeout(resolve, retryDelay * Math.pow(2, attempt)));
            } else {
                notify.notifyError(`Error getting user. ${error.message}`);
                break;
            }
        }
    }

    isLoading.value = false;
}

watch(email, () => notFoundError.value = false);
</script>
