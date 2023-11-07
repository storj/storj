// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog :model-value="shouldSetup" height="87%" width="87%" persistent transition="fade-transition" scrollable>
        <v-card ref="innerContent">
            <!-- Choice step -->
            <choice-step v-if="step === AccountSetupStep.Choice" @next="(s) => step = s" />

            <!-- Business step -->
            <business-step v-if="step === AccountSetupStep.Business" @next="(s) => step = s" />

            <!-- Personal step -->
            <personal-step v-if="step === AccountSetupStep.Personal" @next="(s) => step = s" />

            <!-- Final step -->
            <success-step v-if="step === AccountSetupStep.Success" />
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, ref, watch } from 'vue';
import { VCard, VDialog } from 'vuetify/components';

import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { AccountSetupStep } from '@/types/users';

import ChoiceStep from '@poc/components/dialogs/accountSetupSteps/ChoiceStep.vue';
import BusinessStep from '@poc/components/dialogs/accountSetupSteps/BusinessStep.vue';
import PersonalStep from '@poc/components/dialogs/accountSetupSteps/PersonalStep.vue';
import SuccessStep from '@poc/components/dialogs/accountSetupSteps/SuccessStep.vue';

const userStore = useUsersStore();

const notify = useNotify();

const innerContent = ref<Component | null>(null);
const step = ref<AccountSetupStep>(AccountSetupStep.Choice);

const shouldSetup = computed(() => {
    // settings are fetched on the projects page.
    const onboardingEnd = userStore.state.settings.onboardingEnd;
    if (onboardingEnd) {
        return false;
    }

    if (!userStore.state.user.email) {
        // user has not been fetched yet.
        return false;
    }

    // If the user has a name, they've already completed the account setup.
    return !userStore.userName;
});

watch(innerContent, comp => {
    if (comp) return;
    step.value = AccountSetupStep.Choice;
});
</script>
