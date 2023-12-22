// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row>
            <v-col>
                <PageTitleComponent title="Account Settings" />
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" sm="6">
                <v-card title="Name" variant="outlined" :border="true" rounded="xlg">
                    <v-card-subtitle>
                        {{ user.getFullName() }}
                    </v-card-subtitle>
                    <v-card-text>
                        <v-divider class="mb-4" />
                        <v-btn variant="outlined" color="default" size="small" @click="isChangeNameDialogShown = true">
                            Edit Name
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
            <v-col cols="12" sm="6">
                <v-card title="Email Address" variant="outlined" :border="true" rounded="xlg">
                    <v-card-subtitle>
                        {{ user.email }}
                    </v-card-subtitle>
                    <v-card-text>
                        <v-tooltip
                            activator="parent"
                            location="top"
                        >
                            To change email, please <a href="https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000379291#" target="_blank">contact support</a>.
                        </v-tooltip>
                        <v-divider class="mb-4" />
                        <v-btn variant="outlined" color="default" size="small" disabled>
                            Change Email
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <v-row>
            <v-col>
                <h3 class="mt-5">Security</h3>
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" md="6" lg="4">
                <v-card title="Password" variant="outlined" :border="true" rounded="xlg">
                    <v-card-subtitle>
                        **********
                    </v-card-subtitle>
                    <v-card-text>
                        <v-divider class="mb-4" />
                        <v-btn variant="outlined" color="default" size="small" @click="isChangePasswordDialogShown = true">
                            Change Password
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" md="6" lg="4">
                <v-card title="Two-factor authentication" variant="outlined" :border="true" rounded="xlg">
                    <v-card-subtitle>
                        Improve security by enabling 2FA.
                    </v-card-subtitle>
                    <v-card-text>
                        <v-divider class="mb-4" />
                        <v-btn v-if="!user.isMFAEnabled" size="small" @click="toggleEnableMFADialog">Enable Two-factor</v-btn>
                        <template v-else>
                            <v-btn class="mr-1" variant="outlined" color="default" size="small" @click="toggleRecoveryCodesDialog">Regenerate Recovery Codes</v-btn>
                            <v-btn variant="outlined" color="default" size="small" @click="isDisableMFADialogShown = true">Disable Two-factor</v-btn>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" lg="4">
                <v-card title="Session Timeout" variant="outlined" :border="true" rounded="xlg">
                    <v-card-subtitle>
                        Log out after {{ userSettings.sessionDuration?.shortString ?? Duration.MINUTES_15.shortString }}.
                    </v-card-subtitle>
                    <v-card-text>
                        <v-divider class="mb-4" />
                        <v-btn variant="outlined" color="default" size="small" @click="isSetSessionTimeoutDialogShown = true">
                            Change Timeout
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>
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
    VCardText,
    VCardSubtitle,
    VDivider,
    VBtn,
    VRow,
    VCol,
    VTooltip,
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
