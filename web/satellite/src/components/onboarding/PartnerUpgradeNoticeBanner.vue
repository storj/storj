// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        :model-value="model"
        :title="planInfo.bannerTitle"
        closable
        variant="tonal"
        color="success"
        class="mt-4 mb-4"
        @click:close="dismiss"
    >
        <template #text>
            <p class="mt-2">
                {{ planInfo.bannerText }}
            </p>
            <v-btn
                class="mt-3"
                color="success"
                :append-icon="ArrowRight"
                @click="toggleUpgradeDialog"
            >
                Learn More
            </v-btn>
        </template>
    </v-alert>

    <upgrade-account-dialog
        ref="upgradeDialog"
        v-model="isUpgradeDialogShown"
    />
</template>

<script setup lang="ts">
import { VAlert, VBtn } from 'vuetify/components';
import { ref, watch } from 'vue';
import { ArrowRight } from 'lucide-vue-next';

import { PricingPlanInfo } from '@/types/common';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';

import UpgradeAccountDialog from '@/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';

const usersStore = useUsersStore();

const notify = useNotify();

const isUpgradeDialogShown = ref<boolean>(false);

const upgradeDialog = ref<{ setSecondStep: ()=>void }>();

defineProps<{
    planInfo: PricingPlanInfo,
}>();

const model = defineModel<boolean>({ required: true });

async function dismiss() {
    try {
        const noticeDismissal = { ...usersStore.state.settings.noticeDismissal };
        noticeDismissal.partnerUpgradeBanner = true;
        await usersStore.updateSettings({ noticeDismissal });
        model.value = false;
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_DASHBOARD_PAGE);
    }
}

function toggleUpgradeDialog() {
    // go to the second step, which in this case
    // will be the pricing plan selection step.
    upgradeDialog.value?.setSecondStep();
    isUpgradeDialogShown.value = true;
}

watch(() => [usersStore.state.user.paidTier, isUpgradeDialogShown.value], (value) => {
    if (value[0] && !value[1]) {
        // throttle the banner dismissal for the dialog close animation.
        setTimeout(() => model.value = false, 500);
    }
});
</script>
