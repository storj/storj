// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row justify="center">
            <v-col class="text-center pt-10 pb-4">
                <icon-personal />
                <p class="text-overline mt-2 mb-1">
                    Personal Account
                </p>
                <h2 class="pb-3">Great, almost there.</h2>
                <p>Please complete your account information.</p>
            </v-col>
        </v-row>

        <v-row justify="center">
            <v-col cols="12" sm="8" md="6" lg="4">
                <v-form v-model="formValid">
                    <v-text-field
                        id="Name"
                        v-model="name"
                        :rules="[RequiredRule]"
                        label="Name"
                        placeholder="Enter your name"
                        required
                    />
                    <v-select
                        v-model="useCase"
                        :items="['Video Streaming', 'Media Sharing & Collaboration', 'Large File Distribution', 'Backup/Archive', 'Web3 Storage', 'Other']"
                        label="Use Case (optional)"
                        placeholder="Select your use case"
                        variant="outlined"
                        class="mt-2"
                    />
                </v-form>
            </v-col>
        </v-row>

        <v-row justify="center">
            <v-col cols="6" sm="4" md="3" lg="2">
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
            <v-col cols="6" sm="4" md="3" lg="2">
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
import { RequiredRule } from '@poc/types/common';
import { useLoading } from '@/composables/useLoading';
import { AuthHttpApi } from '@/api/auth';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import IconPersonal from '@poc/components/icons/IconPersonal.vue';

const auth = new AuthHttpApi();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const formValid = ref(false);

const name = ref('');
const useCase = ref<string>();

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
                fullName: name.value,
                storageUseCase: useCase.value,
                haveSalesContact: false,
                isProfessional: false,
            });

            emit('next', AccountSetupStep.Success);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETUP_DIALOG);
        }
    });
}
</script>