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
                v-if="featureFlags.account.updateLimits"
                density="comfortable"
                link rounded="lg"
            >
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
import { VMenu, VList, VListItem, VListItemTitle } from 'vuetify/components';
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { FeatureFlags, UserAccount } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { ROUTES } from '@/router';

import AccountResetMFADialog from '@/components/AccountResetMFADialog.vue';
import AccountDeleteDialog from '@/components/AccountDeleteDialog.vue';
import AccountNewProjectDialog from '@/components/AccountNewProjectDialog.vue';
import AccountGeofenceDialog from '@/components/AccountGeofenceDialog.vue';
import AccountLimitsDialog from '@/components/AccountLimitsDialog.vue';

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

const emit = defineEmits<{
    (e: 'toggleFreeze', user: UserAccount): void;
    (e: 'update', user: UserAccount): void;
}>();

function viewAccount() {
    router.push({ name: ROUTES.Account.name, params: { userID: props.user.id } });
}
</script>
