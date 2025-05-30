// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row justify="center">
            <v-col class="text-center py-4">
                <Building2 height="50" width="50" class="rounded-xlg bg-background pa-3 border" />
                <p class="text-overline mt-2 mb-1">
                    Business Account
                </p>
                <h2 class="pb-3">Experience Storj for your business</h2>
                <p>Tell us a bit about yourself and the company.</p>
            </v-col>
        </v-row>

        <v-form v-model="formValid" @submit.prevent="emit('next')">
            <v-row justify="center">
                <v-col class="text-center">
                    <p class="text-caption text-medium-emphasis">Fields marked with * are required.</p>
                </v-col>
            </v-row>

            <v-row justify="center">
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-text-field
                        id="First Name"
                        v-model="firstName"
                        :rules="[RequiredRule, MaxNameLengthRule]"
                        label="First Name"
                        hide-details="auto"
                        required
                    />
                </v-col>
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-text-field
                        id="Last Name"
                        v-model="lastName"
                        :rules="[MaxNameLengthRule]"
                        label="Last Name"
                        hide-details="auto"
                    />
                </v-col>
            </v-row>
            <v-row justify="center">
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-text-field
                        id="Company Name"
                        v-model="companyName"
                        :rules="[RequiredRule, MaxNameLengthRule]"
                        label="Company Name"
                        hide-details="auto"
                        required
                    />
                </v-col>
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-select
                        id="Job Role"
                        v-model="position"
                        :rules="[RequiredRule]"
                        :items="['Executive/C-Level', 'Director', 'Manager', 'Software Developer', 'Partner', 'Student/Professor', 'Other']"
                        label="Job Role"
                        variant="outlined"
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
                        :items="['Under 25 TB', '25 TB - 50 TB', '51 TB - 150 TB', '151 TB - 250 TB', '251 TB - 500 TB', '501 TB and above']"
                        label="Storage Needs"
                        variant="outlined"
                        hide-details="auto"
                    />
                </v-col>
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-select
                        v-model="useCase"
                        :items="[ 'Active archive', 'Backup & recovery', 'CDN origin', 'Generative AI', 'Media workflows', 'Other']"
                        label="Use Case"
                        variant="outlined"
                        hide-details="auto"
                        @update:model-value="(v) => analyticsStore.eventTriggered(AnalyticsEvent.USE_CASE_SELECTED, { useCase: v })"
                    />
                </v-col>
            </v-row>
            <v-row v-if="useCase === 'Other'" justify="center">
                <v-col cols="12" sm="10" md="8" lg="6">
                    <v-text-field
                        v-model="otherUseCase"
                        label="Specify Other Use Case"
                        variant="outlined"
                        autofocus
                        hide-details="auto"
                    />
                </v-col>
            </v-row>

            <v-row justify="center">
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-checkbox id="sales" v-model="haveSalesContact" hide-details density="compact">
                        <template #label>
                            <p class="text-body-2">I'd like a Sales representative to contact me about my business needs.</p>
                        </template>
                    </v-checkbox>
                </v-col>
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-checkbox id="partnering" v-model="interestedInPartnering" hide-details density="compact">
                        <template #label>
                            <p class="text-body-2">I'm interested in exploring partnership opportunities with Storj for my business.</p>
                        </template>
                    </v-checkbox>
                </v-col>
            </v-row>

            <v-row justify="center" class="mt-4">
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-btn
                        size="large"
                        variant="outlined"
                        :prepend-icon="ChevronLeft"
                        color="default"
                        :disabled="loading"
                        block
                        @click="emit('back')"
                    >
                        Back
                    </v-btn>
                </v-col>
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-btn
                        size="large"
                        :append-icon="ChevronRight"
                        :loading="loading"
                        :disabled="!formValid"
                        block
                        type="submit"
                    >
                        Continue
                    </v-btn>
                </v-col>
            </v-row>
        </v-form>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCheckbox, VCol, VContainer, VForm, VRow, VSelect, VTextField } from 'vuetify/components';
import { ref } from 'vue';
import { Building2, ChevronLeft, ChevronRight } from 'lucide-vue-next';

import { AuthHttpApi } from '@/api/auth';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MaxNameLengthRule, RequiredRule } from '@/types/common';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';

const auth = new AuthHttpApi();

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();

defineProps<{
    loading: boolean,
}>();

const firstName = defineModel<string>('firstName', { required: true });
const lastName = defineModel<string>('lastName', { required: true });
const companyName = defineModel<string>('companyName', { required: true });
const position = defineModel<string | undefined>('position', { required: true });
const employeeCount = defineModel<string | undefined>('employeeCount', { required: true });
const storageNeeds = defineModel<string | undefined>('storageNeeds', { required: true });
const useCase = defineModel<string | undefined>('useCase', { required: true });
const otherUseCase = defineModel<string | undefined>('otherUseCase', { required: true });
const functionalArea = defineModel<string | undefined>('functionalArea', { required: true });
const haveSalesContact = defineModel<boolean>('haveSalesContact', { required: true, default: false });
const interestedInPartnering = defineModel<boolean>('interestedInPartnering', { required: true, default: false });

const emit = defineEmits<{
    (event: 'next'): void,
    (event: 'back'): void,
}>();

const formValid = ref(false);

async function setupAccount() {
    await auth.setupAccount({
        firstName: firstName.value,
        lastName: lastName.value,
        position: position.value,
        companyName: companyName.value,
        employeeCount: employeeCount.value,
        storageNeeds: storageNeeds.value,
        storageUseCase: useCase.value,
        otherUseCase: otherUseCase.value,
        functionalArea: functionalArea.value,
        isProfessional: true,
        haveSalesContact: haveSalesContact.value,
        interestedInPartnering: interestedInPartnering.value,
    }, configStore.state.config.csrfToken);

    analyticsStore.eventTriggered(AnalyticsEvent.BUSINESS_INFO_SUBMITTED);
}

function validate() {
    return formValid.value;
}

defineExpose({
    validate,
    setup: setupAccount,
});
</script>
