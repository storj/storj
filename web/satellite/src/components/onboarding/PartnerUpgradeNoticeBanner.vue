// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        v-if="planInfo"
        :model-value="model"
        :title="planInfo.bannerTitle"
        closable
        variant="tonal"
        type="success"
        rounded="lg"
        class="mt-4 mb-4"
        @click:close="dismiss"
    >
        <template #prepend />
        <template #text>
            <p>
                {{ planInfo.bannerText }}
            </p>
            <v-btn
                class="mt-2"
                color="success"
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

import { PricingPlanInfo } from '@/types/common';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useConfigStore } from '@/store/modules/configStore';
import { PaymentsHttpApi } from '@/api/payments';

import UpgradeAccountDialog from '@/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';

const payments: PaymentsHttpApi = new PaymentsHttpApi();

const configStore = useConfigStore();
const usersStore = useUsersStore();

const notify = useNotify();

const isUpgradeDialogShown = ref<boolean>(false);

const upgradeDialog = ref<{ setSecondStep: ()=>void }>();

const props = defineProps<{
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
