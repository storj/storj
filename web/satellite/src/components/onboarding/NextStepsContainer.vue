// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <template v-if="!hideContainer">
        <div class="d-flex align-center justify-space-between mb-4">
            <div>
                <page-title-component :title="'Welcome ' + user.fullName" />
                <page-subtitle-component subtitle="Your next steps" />
            </div>
            <v-tooltip v-if="shouldShowOnboardStepper" location="bottom" text="Dismiss Onboarding">
                <template #activator="{ props }">
                    <v-btn
                        v-bind="props"
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        :loading="isLoading"
                        @click="dismissOnboarding"
                    />
                </template>
            </v-tooltip>
        </div>

        <onboarding-component v-if="shouldShowOnboardStepper" ref="onboardingStepper" />
        <partner-upgrade-notice-banner v-if="planInfo" v-model="partnerBannerVisible" :plan-info="planInfo" />
    </template>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VBtn, VTooltip } from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { ONBOARDING_STEPPER_STEPS, User } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { PricingPlanInfo } from '@/types/common';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';

import PartnerUpgradeNoticeBanner from '@/components/onboarding/PartnerUpgradeNoticeBanner.vue';
import OnboardingComponent from '@/components/onboarding/OnboardingStepperComponent.vue';
import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';

const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const billingStore = useBillingStore();

const { isLoading, withLoading } = useLoading();

const hideContainer = ref(false);

const onboardingStepper = ref<{ endOnboarding: () => Promise<void> }>();

const user = computed<User>(() => usersStore.state.user);

const userSettings = computed(() => usersStore.state.settings);

const selectedProject = computed(() => projectsStore.state.selectedProject);

const planInfo = computed<PricingPlanInfo | null>(() => billingStore.state.pricingPlanInfo);

const partnerBannerVisible = computed(() => !usersStore.noticeDismissal.partnerUpgradeBanner && billingStore.state.pricingPlansAvailable);

const shouldShowOnboardStepper = computed<boolean>(() => {
    const isNotOwner = selectedProject.value.ownerId !== user.value.id;
    const isNotFirstProject = selectedProject.value.id !== projectsStore.usersFirstProject?.id;

    if (isNotOwner || isNotFirstProject) {
        return false;
    }

    const hasOnboardStep = !!ONBOARDING_STEPPER_STEPS.find(s => s === userSettings.value.onboardingStep);
    return !userSettings.value.onboardingEnd && hasOnboardStep;
});

function dismissOnboarding() {
    withLoading(async () => {
        await onboardingStepper.value?.endOnboarding();
    });
}

// hide container when no content is visible.
watch([partnerBannerVisible, shouldShowOnboardStepper], (value) => {
    // hide container when no content is visible
    hideContainer.value = value.every((v) => !v);
}, { immediate: true });
</script>
