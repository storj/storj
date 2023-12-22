// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row justify="center">
            <v-col class="text-center pt-10 pb-7">
                <icon-business />
                <div class="text-overline mt-2 mb-1">
                    Business Account
                </div>
                <h2 class="pb-3">Experience Storj for your business</h2>
                <p>Tell us a bit about yourself and the company.</p>
            </v-col>
        </v-row>

        <v-form v-model="formValid">
            <v-row justify="center">
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-text-field
                        v-model="firstName"
                        :rules="[RequiredRule]"
                        label="First Name"
                        hide-details="auto"
                        required
                    />
                </v-col>
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-text-field
                        v-model="lastName"
                        label="Last Name"
                        hide-details="auto"
                    />
                </v-col>
            </v-row>
            <v-row justify="center">
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-text-field
                        v-model="companyName"
                        :rules="[RequiredRule]"
                        label="Company Name"
                        hide-details="auto"
                        required
                    />
                </v-col>
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-text-field
                        v-model="position"
                        :rules="[RequiredRule]"
                        label="Job Role"
                        hide-details="auto"
                        required
                    />
                </v-col>
            </v-row>
            <v-row justify="center">
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-select
                        v-model="functionalArea"
                        :items="['Cloud Storage', 'IT Security', 'IT Operations', 'Business', 'Other']"
                        label="Functional Area"
                        variant="outlined"
                        hide-details="auto"
                    />
                </v-col>
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-select
                        v-model="employeeCount"
                        :items="['1-50', '50-1000', '1000+']"
                        label="Number of Employees"
                        variant="outlined"
                        hide-details="auto"
                    />
                </v-col>
            </v-row>
            <v-row justify="center">
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-select
                        v-model="storageNeeds"
                        :items="['Less than 150TB', '150-499TB', '500-999TB', '1PB+']"
                        label="Storage Needs"
                        variant="outlined"
                        hide-details="auto"
                    />
                </v-col>
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-select
                        v-model="useCase"
                        :items="['Video Streaming', 'Media Sharing & Collaboration', 'Large File Distribution', 'Backup/Archive', 'Web3 Storage', 'Other']"
                        label="Use Case"
                        variant="outlined"
                        hide-details="auto"
                    />
                </v-col>
            </v-row>
        </v-form>

        <v-row justify="center" class="mt-4">
            <v-col cols="12" sm="5" md="4" lg="3">
                <v-btn
                    size="large"
                    variant="tonal"
                    :prepend-icon="mdiChevronLeft"
                    color="default"
                    :disabled="isLoading"
                    block
                    @click="emit('next', AccountSetupStep.Choice)"
                >
                    Back
                </v-btn>
            </v-col>
            <v-col cols="12" sm="5" md="4" lg="3">
                <v-btn
                    size="large"
                    :append-icon="mdiChevronRight"
                    :loading="isLoading"
                    :disabled="!formValid"
                    block
                    @click="setupAccount()"
                >
                    Continue
                </v-btn>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCol, VContainer, VForm, VRow, VSelect, VTextField } from 'vuetify/components';
import { ref } from 'vue';
import { mdiChevronLeft, mdiChevronRight } from '@mdi/js';

import { AccountSetupStep } from '@/types/users';
import { AuthHttpApi } from '@/api/auth';
import { useNotify } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { RequiredRule } from '@poc/types/common';

import IconBusiness from '@poc/components/icons/IconBusiness.vue';

const auth = new AuthHttpApi();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const formValid = ref(false);

const firstName = ref('');
const lastName = ref('');
const companyName = ref('');
const position = ref('');
const employeeCount = ref<string>();
const storageNeeds = ref<string>();
const useCase = ref<string>();
const functionalArea = ref<string>();

const emit = defineEmits<{
    next: [AccountSetupStep];
}>();

function setupAccount() {
    withLoading(async () => {
        if (!formValid.value) {
            return;
        }

        try {
            await auth.setupAccount({
                fullName: `${firstName.value} ${lastName.value}`,
                position: position.value,
                employeeCount: employeeCount.value,
                storageNeeds: storageNeeds.value,
                isProfessional: true,
            });

            emit('next', AccountSetupStep.Success);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETUP_DIALOG);
        }
    });
}
</script>
