// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row justify="center">
            <v-col class="text-center py-4">
                <icon-storj-logo v-if="configStore.isDefaultBrand" height="50" width="50" class="rounded-xlg bg-background pa-2 border" />
                <v-img v-else :src="configStore.logo" class="rounded-xlg bg-background pa-2 border mx-auto" height="50" width="50" alt="Logo" />
                <p class="text-overline mt-2 mb-1">
                    Welcome
                </p>
                <h2 class="pb-3">Set up your account</h2>
            </v-col>
        </v-row>

        <v-form v-model="formValid" @submit.prevent="emit('next')">
            <v-row justify="center">
                <v-col cols="12" sm="5" md="4" lg="3">
                    <v-text-field
                        id="Name"
                        v-model="name"
                        :rules="[RequiredRule, MaxNameLengthRule]"
                        label="Name"
                        hide-details="auto"
                        required
                    />
                </v-col>
            </v-row>

            <template v-if="!isMemberAccount">
                <v-row justify="center">
                    <v-col cols="12" sm="5" md="4" lg="3">
                        <v-text-field
                            id="Company Name"
                            v-model="companyName"
                            :rules="[MaxNameLengthRule]"
                            label="Company Name"
                            hide-details="auto"
                        />
                    </v-col>
                </v-row>
                <v-row justify="center">
                    <v-col cols="12" sm="5" md="4" lg="3">
                        <v-select
                            v-model="storageNeeds"
                            :items="Object.values(AccountSetupStorageNeeds)"
                            label="Storage Needs"
                            variant="outlined"
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
                </v-row>
            </template>

            <v-row justify="center" class="mt-4">
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
import { VBtn, VCheckbox, VCol, VContainer, VForm, VRow, VSelect, VTextField, VImg } from 'vuetify/components';
import { computed, ref, watch } from 'vue';
import { ChevronRight } from 'lucide-vue-next';

import { AuthHttpApi } from '@/api/auth';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MaxNameLengthRule, RequiredRule } from '@/types/common';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { AccountSetupStorageNeeds } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';

import IconStorjLogo from '@/components/icons/IconStorjLogo.vue';

const auth = new AuthHttpApi();

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();

defineProps<{
    loading: boolean,
}>();

const name = defineModel<string>('name', { required: true });
const companyName = defineModel<string>('companyName', { required: true });
const storageNeeds = defineModel<AccountSetupStorageNeeds | undefined>('storageNeeds', { required: true });
const haveSalesContact = defineModel<boolean>('haveSalesContact', { required: true, default: false });

const emit = defineEmits<{
    (event: 'next'): void,
}>();

const formValid = ref<boolean>(false);

const isMemberAccount = computed<boolean>(() => usersStore.state.user.isMember);

async function setupAccount(): Promise<void> {
    const hasCompany = companyName.value !== '';
    const [firstName, lastName] = name.value.split(' ', 2);

    await auth.setupAccount({
        fullName: name.value,
        firstName: firstName,
        lastName: lastName,
        companyName: companyName.value,
        storageNeeds: storageNeeds.value,
        isProfessional: hasCompany,
        haveSalesContact: haveSalesContact.value,
        interestedInPartnering: false,
    }, configStore.state.config.csrfToken);

    if (hasCompany) {
        analyticsStore.eventTriggered(AnalyticsEvent.BUSINESS_INFO_SUBMITTED);
    } else {
        analyticsStore.eventTriggered(AnalyticsEvent.PERSONAL_INFO_SUBMITTED);
    }
}

function validate(): boolean {
    return formValid.value;
}

watch(storageNeeds, val => {
    haveSalesContact.value = val === AccountSetupStorageNeeds.OVER_1PB;
});

defineExpose({
    validate,
    setup: setupAccount,
});
</script>
