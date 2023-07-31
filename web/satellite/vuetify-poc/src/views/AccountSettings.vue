// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <!-- <v-breadcrumbs :items="['My Account', 'Settings']" class="pl-0"></v-breadcrumbs> -->

        <h1 class="text-h5 font-weight-bold mb-2">Settings</h1>

        <v-card
            variant="flat"
            :border="true"
            class="mx-auto my-6"
        >
            <v-list lines="three">
                <v-list-subheader class="mb-2">Profile</v-list-subheader>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Name</v-list-item-title>

                    <v-list-item-subtitle>
                        {{ user.getFullName() }}
                    </v-list-item-subtitle>

                    <template #append>
                        <v-list-item-action>
                            <v-btn variant="outlined" color="default" size="small" @click="isChangeNameDialogShown = true">
                                Edit Name
                            </v-btn>
                        </v-list-item-action>
                    </template>
                </v-list-item>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Email</v-list-item-title>

                    <v-list-item-subtitle>
                        {{ user.email }}
                    </v-list-item-subtitle>

                    <!-- <template v-slot:append>
                         <v-list-item-action>
                            <v-btn>Change Email</v-btn>
                        </v-list-item-action>
                    </template> -->
                </v-list-item>
            </v-list>
        </v-card>
        <v-card
            variant="flat"
            :border="true"
            class="mx-auto my-6"
        >
            <v-list lines="three">
                <v-list-subheader class="mb-2">Security</v-list-subheader>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Password</v-list-item-title>

                    <v-list-item-subtitle>
                        **********
                    </v-list-item-subtitle>

                    <template #append>
                        <v-list-item-action>
                            <v-btn variant="outlined" color="default" size="small" @click="isChangePasswordDialogShown = true">
                                Change Password
                            </v-btn>
                        </v-list-item-action>
                    </template>
                </v-list-item>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Two-factor authentication</v-list-item-title>

                    <v-list-item-subtitle>
                        Improve account security by enabling 2FA.
                    </v-list-item-subtitle>

                    <template #append>
                        <v-list-item-action>
                            <v-btn size="small">Enable Two-factor</v-btn>
                        </v-list-item-action>
                    </template>
                </v-list-item>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Session Timeout</v-list-item-title>

                    <v-list-item-subtitle>
                        Set timeout to log you out for inactivity.
                    </v-list-item-subtitle>

                    <template #append>
                        <v-list-item-action>
                            <v-btn variant="outlined" color="default" size="small">Set Timeout</v-btn>
                        </v-list-item-action>
                    </template>
                </v-list-item>
            </v-list>
        </v-card>

        <v-card
            variant="flat"
            :border="true"
            class="mx-auto my-6"
        >
            <v-list lines="three" select-strategy="classic">
                <v-list-subheader class="mb-2">Notifications</v-list-subheader>

                <v-divider />

                <v-list-item value="notifications" color="default">
                    <template #append="{ isActive }">
                        <v-list-item-action start>
                            <v-checkbox-btn :model-value="isActive" />
                        </v-list-item-action>
                    </template>

                    <v-list-item-title>Product newsletter</v-list-item-title>

                    <v-list-item-subtitle>
                        Notify me about product updates.
                    </v-list-item-subtitle>
                </v-list-item>
            </v-list>
        </v-card>
    </v-container>

    <ChangePasswordDialog
        v-model="isChangePasswordDialogShown"
    />

    <ChangeNameDialog
        v-model="isChangeNameDialogShown"
    />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VContainer,
    VCard,
    VList,
    VListSubheader,
    VDivider,
    VListItem,
    VListItemTitle,
    VListItemSubtitle,
    VListItemAction,
    VBtn,
    VCheckboxBtn,
} from 'vuetify/components';

import { useUsersStore } from '@/store/modules/usersStore';
import { User } from '@/types/users';

import ChangePasswordDialog from '@poc/components/dialogs/ChangePasswordDialog.vue';
import ChangeNameDialog from '@poc/components/dialogs/ChangeNameDialog.vue';

const usersStore = useUsersStore();

const isChangePasswordDialogShown = ref<boolean>(false);
const isChangeNameDialogShown = ref<boolean>(false);

/**
 * Returns user entity from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});
</script>
