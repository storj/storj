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

        <v-form v-model="formValid" @submit.prevent="setupAccount">
            <v-row justify="center">
                <v-col cols="12" sm="6" md="5" lg="4" class="py-0">
                    <p>Fields marked with an * are required</p>
                </v-col>
                <v-col cols="12" sm="4" md="3" lg="2" />
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

            <v-row justify="center">
                <v-col cols="12" sm="6" md="5" lg="4">
                    <v-checkbox id="sales" v-model="haveSalesContact" hide-details density="compact">
                        <template #label>
                            <p class="text-body-2">Please have the Sales Team contact me</p>
                        </template>
                    </v-checkbox>
                    <v-checkbox id="partnering" v-model="interestedInPartnering" hide-details density="compact">
                        <template #label>
                            <p class="text-body-2">I am interested in partnering with Storj</p>
                        </template>
                    </v-checkbox>
                </v-col>
                <v-col v-if="smAndUp" cols="12" sm="4" md="3" lg="2" />
            </v-row>

            <v-row justify="center" class="mt-4">
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-btn
                        size="large"
                        variant="outlined"
                        :prepend-icon="mdiChevronLeft"
                        color="default"
                        :disabled="isLoading"
                        block
                        @click="emit('back')"
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
import { mdiChevronLeft, mdiChevronRight } from '@mdi/js';
import { useDisplay } from 'vuetify';

import { OnboardingStep } from '@/types/users';
import { AuthHttpApi } from '@/api/auth';
import { useNotify } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MaxNameLengthRule, RequiredRule } from '@/types/common';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import IconBusiness from '@/components/icons/IconBusiness.vue';

const auth = new AuthHttpApi();

const analyticsStore = useAnalyticsStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();
const { smAndUp } = useDisplay();

const formValid = ref(false);

const firstName = ref('');
const lastName = ref('');
const companyName = ref('');
const position = ref<string>();
const employeeCount = ref<string>();
const storageNeeds = ref<string>();
const useCase = ref<string>();
const functionalArea = ref<string>();
const haveSalesContact = ref(false);
const interestedInPartnering = ref(false);

const emit = defineEmits<{
    (event: 'next'): void,
    (event: 'back'): void,
}>();

function setupAccount() {
    withLoading(async () => {
        if (!formValid.value) {
            return;
        }

        try {
            await auth.setupAccount({
                firstName: firstName.value,
                lastName: lastName.value,
                position: position.value,
                companyName: companyName.value,
                employeeCount: employeeCount.value,
                storageNeeds: storageNeeds.value,
                storageUseCase: useCase.value,
                functionalArea: functionalArea.value,
                isProfessional: true,
                haveSalesContact: haveSalesContact.value,
                interestedInPartnering: interestedInPartnering.value,
            });

            analyticsStore.eventTriggered(AnalyticsEvent.BUSINESS_INFO_SUBMITTED);
            emit('next');
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ONBOARDING_FORM);
        }
    });
}
</script>
