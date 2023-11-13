// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row>
            <v-col>
                <PageTitleComponent title="Account Settings" />
            </v-col>
        </v-row>

        <v-card
            variant="flat"
            :border="true"
            class="mx-auto mt-2 my-6"
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
                        <v-list-item-action justify="end" class="flex-column flex-sm-row align-end">
                            <v-btn v-if="!user.isMFAEnabled" size="small" @click="toggleEnableMFADialog">Enable Two-factor</v-btn>
                            <template v-else>
                                <v-btn class="mb-1 mb-sm-0 mr-sm-1" variant="outlined" color="default" size="small" @click="toggleRecoveryCodesDialog">Regenerate Recovery Codes</v-btn>
                                <v-btn variant="outlined" color="default" size="small" @click="isDisableMFADialogShown = true">Disable Two-factor</v-btn>
                            </template>
                        </v-list-item-action>
                    </template>
                </v-list-item>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Session Timeout</v-list-item-title>

                    <v-list-item-subtitle>
                        {{ userSettings.sessionDuration?.shortString ?? Duration.MINUTES_15.shortString }}
                    </v-list-item-subtitle>

                    <template #append>
                        <v-list-item-action>
                            <v-btn variant="outlined" color="default" size="small" @click="isSetSessionTimeoutDialogShown = true">
                                Set Timeout
                            </v-btn>
                        </v-list-item-action>
                    </template>
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

    <EnableMFADialog
        v-model="isEnableMFADialogShown"
    />

    <DisableMFADialog
        v-model="isDisableMFADialogShown"
    />

    <MFACodesDialog
        v-model="isRecoveryCodesDialogShown"
    />

    <SetSessionTimeoutDialog
        v-model="isSetSessionTimeoutDialogShown"
    />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
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
    VRow,
    VCol,
} from 'vuetify/components';

import { User, UserSettings } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { Duration } from '@/utils/time';

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import ChangePasswordDialog from '@poc/components/dialogs/ChangePasswordDialog.vue';
import ChangeNameDialog from '@poc/components/dialogs/ChangeNameDialog.vue';
import EnableMFADialog from '@poc/components/dialogs/EnableMFADialog.vue';
import DisableMFADialog from '@poc/components/dialogs/DisableMFADialog.vue';
import MFACodesDialog from '@poc/components/dialogs/MFACodesDialog.vue';
import SetSessionTimeoutDialog from '@poc/components/dialogs/SetSessionTimeoutDialog.vue';

const usersStore = useUsersStore();
const notify = useNotify();

const isChangePasswordDialogShown = ref<boolean>(false);
const isChangeNameDialogShown = ref<boolean>(false);
const isEnableMFADialogShown = ref<boolean>(false);
const isDisableMFADialogShown = ref<boolean>(false);
const isRecoveryCodesDialogShown = ref<boolean>(false);
const isSetSessionTimeoutDialogShown = ref<boolean>(false);

/**
 * Returns user entity from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});

/**
 * Returns user settings from store.
 */
const userSettings = computed((): UserSettings => {
    return usersStore.state.settings as UserSettings;
});

async function toggleEnableMFADialog() {
    try {
        await usersStore.generateUserMFASecret();
        isEnableMFADialogShown.value = true;
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETTINGS_AREA);
    }
}

async function toggleRecoveryCodesDialog() {
    try {
        isRecoveryCodesDialogShown.value = true;
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETTINGS_AREA);
    }
}

onMounted(() => {
    Promise.all([
        usersStore.getUser(),
        usersStore.getSettings(),
    ]);
});
</script>
