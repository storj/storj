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

            <v-list-item
                v-if="hasAnyUpdateFlags"
                density="comfortable" link
                rounded="lg"
                @click="emit('update', user)"
            >
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Update Account
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.account.updatePlacement" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Placement
                    <AccountGeofenceDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item
                v-if="featureFlags.account.updateLimits && !user.freezeStatus"
                density="comfortable"
                link rounded="lg"
                @click="emit('updateLimits', user)"
            >
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Change Limits
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.project.create" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    New Project
                    <AccountNewProjectDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.account.disableMFA" density="comfortable" link rounded="lg">
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

            <v-list-item
                v-if="featureFlags.account.delete"
                density="comfortable"
                rounded="lg" link
                base-color="error"
                @click="user.hasUnpaidInvoices || hasActiveProjects ? ()=>{} : emit('delete', user)"
            >
                <v-list-item-title
                    v-if="cantDeleteText"
                    v-tooltip="cantDeleteText"
                    class="text-body-2 font-weight-medium text-disabled"
                >
                    Delete
                </v-list-item-title>
                <v-list-item-title v-else class="text-body-2 font-weight-medium">
                    Delete
                </v-list-item-title>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script setup lang="ts">
import { VMenu, VList, VListItem, VListItemTitle } from 'vuetify/components';
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { FeatureFlags, UserAccount } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { ROUTES } from '@/router';

import AccountResetMFADialog from '@/components/AccountResetMFADialog.vue';
import AccountNewProjectDialog from '@/components/AccountNewProjectDialog.vue';
import AccountGeofenceDialog from '@/components/AccountGeofenceDialog.vue';

const appStore = useAppStore();
const router = useRouter();

const props = defineProps<{
    user: UserAccount;
}>();

const featureFlags = computed(() => appStore.state.settings.admin.features as FeatureFlags);

const hasAnyUpdateFlags = computed(() => {
    return featureFlags.value.account.updateName ||
      featureFlags.value.account.updateEmail ||
      featureFlags.value.account.updateKind ||
      featureFlags.value.account.updateStatus ||
      featureFlags.value.account.updateUserAgent;
});

const isCurrentRouteViewAccount = computed(() => {
    return router.currentRoute.value.name === ROUTES.Account.name;
});

const hasActiveProjects = computed(() =>  props.user.projects?.some(p => p.active) ?? false);

const cantDeleteText = computed(() => {
    if (props.user.hasUnpaidInvoices && hasActiveProjects.value) {
        return 'User has unpaid invoices and active projects so cannot be deleted';
    }
    if (props.user.hasUnpaidInvoices) {
        return 'User has unpaid invoices so cannot be deleted';
    }
    if (hasActiveProjects.value) {
        return 'User has active projects so cannot be deleted';
    }
    return '';
});

const emit = defineEmits<{
    (e: 'toggleFreeze', user: UserAccount): void;
    (e: 'update', user: UserAccount): void;
    (e: 'updateLimits', user: UserAccount): void;
    (e: 'delete', user: UserAccount): void;
}>();

function viewAccount() {
    router.push({ name: ROUTES.Account.name, params: { userID: props.user.id } });
}
</script>
