// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="pb-14">
        <v-row>
            <v-col>
                <trial-expiration-banner v-if="isTrialExpirationBanner" :expired="isExpired" />

                <PageTitleComponent title="Account Settings" />
                <PageSubtitleComponent subtitle="Manage your profile, security preferences, and account details" />
            </v-col>
        </v-row>

        <v-row>
            <v-col>
                <h3 class="mt-5">Profile Information</h3>
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" sm="6" lg="4">
                <v-card title="Name" class="pa-2">
                    <v-card-text>
                        <v-chip variant="tonal" color="primary" size="small" class="font-weight-bold">
                            {{ user.getFullName() }}
                        </v-chip>
                        <v-divider class="my-4 border-0" />
                        <v-btn variant="outlined" color="default" :prepend-icon="UserPen" @click="isChangeNameDialogShown = true">
                            Edit Name
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
            <v-col cols="12" sm="6" lg="4">
                <v-card title="Email Address" class="pa-2">
                    <v-card-text>
                        <v-chip color="primary" variant="tonal" size="small" class="font-weight-bold font-family-mono">
                            {{ user.email }}
                        </v-chip>
                        <template v-if="!user.externalID">
                            <v-divider class="my-4 border-0" />
                            <v-btn
                                v-if="changeEmailEnabled"
                                variant="outlined"
                                color="default"
                                :prepend-icon="MailPlus"
                                @click="isChangeEmailDialogShown = true"
                            >
                                Change Email
                            </v-btn>
                            <div v-else>
                                <v-tooltip
                                    activator="parent"
                                    location="top"
                                >
                                    To change email, please <a :href="supportLink" target="_blank" rel="noopener noreferrer">contact support</a>.
                                </v-tooltip>
                                <v-btn variant="outlined" color="default" disabled>
                                    Change Email
                                </v-btn>
                            </div>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>
            <v-col v-if="billingEnabled || user.isNFR" cols="12" sm="6" lg="4">
                <v-card title="Account Type" class="pa-2">
                    <v-card-text>
                        <v-chip
                            class="font-weight-bold"
                            :color="isPaidTier ? 'success' : user.isNFR ? 'warning' : 'info'"
                            variant="tonal"
                            size="small"
                        >
                            {{ user.kind.name }}
                        </v-chip>
                        <template v-if="billingEnabled">
                            <v-divider class="my-4 border-0" />
                            <v-btn v-if="isPaidTier" variant="outlined" color="default" :to="ROUTES.Billing.path" :append-icon="ArrowRight">
                                View Billing
                            </v-btn>
                            <v-btn v-else color="primary" :append-icon="ArrowRight" @click="appStore.toggleUpgradeFlow(true)">
                                Upgrade
                            </v-btn>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <v-row>
            <v-col>
                <h3 class="mt-5">Security Settings</h3>
            </v-col>
        </v-row>

        <v-row>
            <v-col v-if="!user.externalID" cols="12" sm="6" lg="4">
                <v-card title="Password" variant="outlined" class="pa-2">
                    <v-card-subtitle>
                        ••••••••••
                    </v-card-subtitle>
                    <v-card-text>
                        <v-btn variant="outlined" color="default" :prepend-icon="Lock" @click="isChangePasswordDialogShown = true">
                            Change Password
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col v-if="!user.externalID" cols="12" sm="6" lg="4">
                <v-card title="Two-factor authentication" class="pa-2">
                    <v-card-subtitle>
                        Improve security by enabling 2FA.
                    </v-card-subtitle>
                    <v-card-text>
                        <v-btn v-if="!user.isMFAEnabled" :prepend-icon="ShieldCheck" @click="toggleEnableMFADialog">Enable Two-factor</v-btn>
                        <template v-else>
                            <v-btn class="mr-1" variant="outlined" color="default" @click="toggleRecoveryCodesDialog">Regenerate Recovery Codes</v-btn>
                            <v-btn variant="outlined" color="default" :prepend-icon="ShieldOff" @click="isDisableMFADialogShown = true">Disable Two-factor</v-btn>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <v-card title="Session Timeout" class="pa-2">
                    <v-card-subtitle>
                        Currently set to {{ userSettings.sessionDuration?.shortString ?? Duration.MINUTES_15.shortString }}.
                    </v-card-subtitle>
                    <v-card-text>
                        <v-btn variant="outlined" color="default" :prepend-icon="Timer" @click="isSetSessionTimeoutDialogShown = true">
                            Change Timeout
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <v-card title="Passphrase Preference" class="pa-2">
                    <v-card-subtitle>
                        {{ userSettings.passphrasePrompt ? 'Ask for passphrase when opening a project.' : 'Only ask for passphrase when necessary.' }}
                    </v-card-subtitle>
                    <v-card-text>
                        <v-btn variant="outlined" color="default" @click="isSetPassphrasePromptDialogShown = true">
                            {{ userSettings.passphrasePrompt ? 'Disable' : 'Enable' }}
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <template v-if="deleteAccountEnabled && !user.externalID">
            <v-row>
                <v-col>
                    <h3 class="mt-5">Danger Zone</h3>
                </v-col>
            </v-row>

            <v-row>
                <v-col cols="12" sm="6" lg="4">
                    <v-card title="Delete Account" class="pa-2">
                        <v-card-subtitle>
                            Delete all of your own projects and data.
                        </v-card-subtitle>
                        <v-card-text>
                            <v-btn variant="outlined" color="error" :prepend-icon="UserRoundX" @click="isAccountDeleteDialogShown = true">
                                Delete Account
                            </v-btn>
                        </v-card-text>
                    </v-card>
                </v-col>
            </v-row>
        </template>

        <v-row v-if="sessionsViewEnabled">
            <v-col>
                <h3 class="my-5">Active Sessions</h3>
                <active-sessions-table />
            </v-col>
        </v-row>
    </v-container>

    <AccountEmailChangeDialog
        v-if="changeEmailEnabled"
        v-model="isChangeEmailDialogShown"
    />

    <AccountDeleteDialog
        v-if="deleteAccountEnabled"
        v-model="isAccountDeleteDialogShown"
    />

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

    <SetPassphrasePromptDialog
        v-model="isSetPassphrasePromptDialogShown"
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
    VChip,
} from 'vuetify/components';
import { ArrowRight, ShieldCheck, ShieldOff, Lock, Timer, MailPlus, UserPen, UserRoundX } from 'lucide-vue-next';

import { User, UserSettings } from '@/types/users';
import { useAppStore } from '@/store/modules/appStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { Duration } from '@/utils/time';
import { ROUTES } from '@/router';
import { useConfigStore } from '@/store/modules/configStore';
import { usePreCheck } from '@/composables/usePreCheck';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import ChangePasswordDialog from '@/components/dialogs/ChangePasswordDialog.vue';
import ChangeNameDialog from '@/components/dialogs/ChangeNameDialog.vue';
import EnableMFADialog from '@/components/dialogs/EnableMFADialog.vue';
import DisableMFADialog from '@/components/dialogs/DisableMFADialog.vue';
import MFACodesDialog from '@/components/dialogs/MFACodesDialog.vue';
import SetSessionTimeoutDialog from '@/components/dialogs/SetSessionTimeoutDialog.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';
import SetPassphrasePromptDialog from '@/components/dialogs/SetPassphrasePromptDialog.vue';
import AccountEmailChangeDialog from '@/components/dialogs/AccountEmailChangeDialog.vue';
import AccountDeleteDialog from '@/components/dialogs/AccountDeleteDialog.vue';
import ActiveSessionsTable from '@/components/ActiveSessionsTable.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';

const appStore = useAppStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();

const notify = useNotify();
const { isTrialExpirationBanner, isExpired } = usePreCheck();

const isChangePasswordDialogShown = ref<boolean>(false);
const isChangeNameDialogShown = ref<boolean>(false);
const isEnableMFADialogShown = ref<boolean>(false);
const isDisableMFADialogShown = ref<boolean>(false);
const isRecoveryCodesDialogShown = ref<boolean>(false);
const isSetSessionTimeoutDialogShown = ref<boolean>(false);
const isSetPassphrasePromptDialogShown = ref<boolean>(false);
const isChangeEmailDialogShown = ref<boolean>(false);
const isAccountDeleteDialogShown = ref<boolean>(false);

const supportLink = computed<string>(() => `${configStore.supportUrl}?ticket_form_id=360000379291#`);

/**
 * Returns user entity from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});

/**
 * Whether billing features should be enabled
 */
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(user.value));

/**
 * Whether change email feature should be enabled
 */
const changeEmailEnabled = computed<boolean>(() => configStore.state.config.emailChangeFlowEnabled);

/**
 * Whether delete account feature should be enabled
 */
const deleteAccountEnabled = computed<boolean>(() => configStore.state.config.selfServeAccountDeleteEnabled);

/**
 * Whether active sessions table view should be enabled.
 */
const sessionsViewEnabled = computed<boolean>(() => configStore.state.config.activeSessionsViewEnabled);

/**
 * Returns user settings from store.
 */
const userSettings = computed((): UserSettings => {
    return usersStore.state.settings as UserSettings;
});

/**
 * Returns user's paid tier status from store.
 */
const isPaidTier = computed<boolean>(() => {
    return user.value.isPaid;
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
