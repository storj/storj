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
                        icon="$close"
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
        <partner-upgrade-notice-banner v-if="partnerBannerVisible" v-model="partnerBannerVisible" :plan-info="planInfo" />
    </template>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref, watch } from 'vue';
import { VBtn, VTooltip } from 'vuetify/components';

import { ONBOARDING_STEPPER_STEPS, User } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { PricingPlanInfo } from '@/types/common';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { PaymentsHttpApi } from '@/api/payments';
import { useConfigStore } from '@/store/modules/configStore';

import PartnerUpgradeNoticeBanner from '@/components/onboarding/PartnerUpgradeNoticeBanner.vue';
import OnboardingComponent from '@/components/onboarding/OnboardingStepperComponent.vue';
import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';

const payments: PaymentsHttpApi = new PaymentsHttpApi();

const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const partnerBannerVisible = ref(false);
const hideContainer = ref(false);
const planInfo = ref<PricingPlanInfo>();

const onboardingStepper = ref<{ endOnboarding: () => Promise<void> }>();

const user = computed<User>(() => usersStore.state.user);

const userSettings = computed(() => usersStore.state.settings);

const selectedProject = computed(() => projectsStore.state.selectedProject);

const shouldShowOnboardStepper = computed<boolean>(() => {
    if (!configStore.state.config.onboardingStepperEnabled) {
        return false;
    }
    if (selectedProject.value.ownerId !== user.value.id) {
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

function getShouldShowPartnerBanner() {
    withLoading(async () => {
        if (!configStore.state.config.billingFeaturesEnabled
        || !configStore.state.config.pricingPackagesEnabled
        || usersStore.noticeDismissal.partnerUpgradeBanner) {
            return;
        }
        const user: User = usersStore.state.user;
        if (user.paidTier || !user.partner) {
            return;
        }

        try {
            const hasPkg = await payments.pricingPackageAvailable();
            if (!hasPkg) {
                return;
            }
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_DASHBOARD_PAGE);
            return;
        }

        let config;
        try {
            config = (await import('@/configs/pricingPlanConfig.json')).default;
        } catch {
            return;
        }

        planInfo.value = config[user.partner] as PricingPlanInfo;
        if (!planInfo.value) {
            return;
        }
        partnerBannerVisible.value = true;
    });
}

onBeforeMount(() => {
    getShouldShowPartnerBanner();
});

// hide container when no content is visible.
watch([partnerBannerVisible, shouldShowOnboardStepper], (value) => {
    // hide container when no content is visible
    hideContainer.value = value.every((v) => !v);
}, { immediate: true });
</script>
