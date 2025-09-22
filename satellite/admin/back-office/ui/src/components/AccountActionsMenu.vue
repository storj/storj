// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-menu activator="parent">
        <v-list class="pa-2">
            <v-list-item v-if="featureFlags.account.view && !isCurrentRouteViewAccount" density="comfortable" link rounded="lg" base-color="info" @click="viewAccount">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    View Account
                </v-list-item-title>
            </v-list-item>

            <v-divider v-if="featureFlags.account.updateInfo || featureFlags.account.updateStatus || featureFlags.account.updateValueAttribution || featureFlags.account.updatePlacement || featureFlags.account.updateLimits || featureFlags.project.create " class="my-2" />

            <v-list-item v-if="featureFlags.account.updateInfo" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Edit Account
                    <AccountInformationDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.account.updateStatus" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Status
                    <AccountStatusDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.account.updateValueAttribution" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Value
                    <AccountUserAgentsDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.account.updatePlacement" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Placement
                    <AccountGeofenceDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.account.updateLimits" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Change Limits
                    <AccountLimitsDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.project.create" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    New Project
                    <AccountNewProjectDialog />
                </v-list-item-title>
            </v-list-item>

            <v-divider v-if="featureFlags.account.resetMFA || featureFlags.account.suspend || featureFlags.account.delete" class="my-2" />

            <v-list-item v-if="featureFlags.account.resetMFA" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Reset MFA
                    <AccountResetMFADialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item
                v-if="featureFlags.account.suspend || featureFlags.account.unsuspend"
                density="comfortable" link rounded="lg" base-color="warning"
            >
                <v-list-item-title class="text-body-2 font-weight-medium" @click="emit('toggleFreeze', user)">
                    {{ user.freezeStatus ? "Unfreeze" : "Freeze" }}
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.account.delete" density="comfortable" link rounded="lg" base-color="error">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Delete
                    <AccountDeleteDialog />
                </v-list-item-title>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script setup lang="ts">
import { VMenu, VList, VListItem, VListItemTitle, VDivider } from 'vuetify/components';
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { FeatureFlags, UserAccount } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { ROUTES } from '@/router';

import AccountInformationDialog from '@/components/AccountInformationDialog.vue';
import AccountStatusDialog from '@/components/AccountStatusDialog.vue';
import AccountResetMFADialog from '@/components/AccountResetMFADialog.vue';
import AccountDeleteDialog from '@/components/AccountDeleteDialog.vue';
import AccountNewProjectDialog from '@/components/AccountNewProjectDialog.vue';
import AccountGeofenceDialog from '@/components/AccountGeofenceDialog.vue';
import AccountUserAgentsDialog from '@/components/AccountUserAgentsDialog.vue';
import AccountLimitsDialog from '@/components/AccountLimitsDialog.vue';

const appStore = useAppStore();
const router = useRouter();

const props = defineProps<{
    user: UserAccount;
}>();

const featureFlags = computed(() => appStore.state.settings.admin.features as FeatureFlags);

const isCurrentRouteViewAccount = computed(() => {
    return router.currentRoute.value.name === ROUTES.Account.name;
});

const emit = defineEmits<{
    (e: 'toggleFreeze', user: UserAccount): void;
}>();

function viewAccount() {
    router.push({ name: ROUTES.Account.name, params: { email: props.user.email } });
}
</script>
