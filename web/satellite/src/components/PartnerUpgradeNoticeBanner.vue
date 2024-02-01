// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        v-if="!isLoading && planInfo"
        :model-value="model"
        :title="planInfo.bannerTitle"
        closable
        variant="tonal"
        type="success"
        rounded="lg"
        class="mt-2 mb-4"
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
import { computed, onBeforeMount, ref, watch } from 'vue';

import { User } from '@/types/users';
import { PricingPlanInfo } from '@/types/common';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useConfigStore } from '@/store/modules/configStore';
import { PaymentsHttpApi } from '@/api/payments';
import { useLoading } from '@/composables/useLoading';

import UpgradeAccountDialog from '@/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';

const payments: PaymentsHttpApi = new PaymentsHttpApi();

const configStore = useConfigStore();
const usersStore = useUsersStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const isUpgradeDialogShown = ref<boolean>(false);
const planInfo = ref<PricingPlanInfo>();

const upgradeDialog = ref<{ setSecondStep: ()=>void }>();

const props = defineProps<{
    modelValue: boolean,
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const emit = defineEmits<{
    (event: 'update:modelValue', value: boolean): void,
}>();

async function dismiss() {
    model.value = false;
    try {
        const noticeDismissal = { ...usersStore.state.settings.noticeDismissal };
        noticeDismissal.partnerUpgradeBanner = true;
        await usersStore.updateSettings({ noticeDismissal });
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

onBeforeMount(() => {
    withLoading(async () => {
        if (!configStore.state.config.billingFeaturesEnabled
            || !configStore.state.config.pricingPackagesEnabled
            || usersStore.noticeDismissal.partnerUpgradeBanner) {
            model.value = false;
            return;
        }
        const user: User = usersStore.state.user;
        if (user.paidTier || !user.partner) {
            model.value = false;
            return;
        }

        try {
            model.value = await payments.pricingPackageAvailable();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_DASHBOARD_PAGE);
            model.value = false;
            return;
        }

        let config;
        try {
            config = (await import('@/configs/pricingPlanConfig.json')).default;
        } catch {
            model.value = false;
            return;
        }

        planInfo.value = config[user.partner] as PricingPlanInfo;
        if (!planInfo.value) {
            model.value = false;
            return;
        }
        model.value = true;
    });
});

watch(() => [usersStore.state.user.paidTier, isUpgradeDialogShown.value], (value) => {
    if (value[0] && !value[1]) {
        // throttle the banner dismissal for the dialog close animation.
        setTimeout(() => model.value = false, 500);
    }
});
</script>
