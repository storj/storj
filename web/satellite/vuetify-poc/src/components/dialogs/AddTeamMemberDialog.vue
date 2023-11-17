// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-overlay v-model="model" persistent />

    <v-dialog
        :model-value="model && !isUpgradeDialogShown"
        width="auto"
        max-width="420px"
        transition="fade-transition"
        :persistent="isLoading"
        :scrim="false"
        @update:model-value="v => model = v"
    >
        <v-card rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <template #prepend>
                    <img class="d-block" src="@/../static/images/team/teamMembers.svg" alt="Team members">
                </template>

                <v-card-title class="font-weight-bold">
                    {{ needsUpgrade ? 'Upgrade to Pro' : 'Add Member' }}
                </v-card-title>

                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-form v-model="valid" class="pa-7 pb-4" @submit.prevent="onPrimaryClick">
                <v-row>
                    <v-col v-if="needsUpgrade">
                        <p class="mb-4">Upgrade now to unlock collaboration and bring your team together in this project.</p>
                    </v-col>
                    <template v-else>
                        <v-col cols="12">
                            <p class="mb-5">Invite a team member to join you in this project.</p>
                            <v-alert
                                variant="tonal"
                                color="info"
                                title="Important Information"
                                text="All team members should use the same passphrase to access the same data."
                                rounded="lg"
                                density="comfortable"
                                border
                            />
                        </v-col>
                        <v-col cols="12">
                            <v-text-field
                                v-model="email"
                                variant="outlined"
                                :rules="emailRules"
                                label="Enter e-mail"
                                hint="Members will have read & write permissions."
                                required
                                autofocus
                                class="my-2"
                            />
                        </v-col>
                    </template>
                </v-row>
            </v-form>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :loading="isLoading"
                            :append-icon="needsUpgrade ? 'mdi-arrow-right' : undefined"
                            @click="onPrimaryClick"
                        >
                            {{ needsUpgrade ? 'Continue' : 'Send Invite' }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <upgrade-account-dialog
        :scrim="false"
        :model-value="model && isUpgradeDialogShown"
        @update:model-value="v => model = isUpgradeDialogShown = v"
    />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VBtn,
    VDivider,
    VForm,
    VRow,
    VCol,
    VAlert,
    VTextField,
    VCardActions,
    VOverlay,
} from 'vuetify/components';

import { RequiredRule, ValidationRule } from '@poc/types/common';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useNotify } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';

import UpgradeAccountDialog from '@poc/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';

const props = defineProps<{
    modelValue: boolean;
    projectId: string;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean];
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const usersStore = useUsersStore();
const analyticsStore = useAnalyticsStore();
const pmStore = useProjectMembersStore();
const configStore = useConfigStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const valid = ref<boolean>(false);
const email = ref<string>('');
const isUpgradeDialogShown = ref<boolean>(false);

const emailRules: ValidationRule<string>[] = [
    RequiredRule,
    v => ((/.+@.+\..+/.test(v)) || 'E-mail must be valid.'),
];

/**
 * Returns whether the user should upgrade to pro tier before inviting.
 */
const needsUpgrade = computed<boolean>(() => {
    const config = configStore.state.config;
    return config.billingFeaturesEnabled && !(usersStore.state.user.paidTier || config.freeTierInvitesEnabled);
});

/**
 * Handles primary button click.
 */
async function onPrimaryClick(): Promise<void> {
    if (needsUpgrade.value) {
        isUpgradeDialogShown.value = true;
        return;
    }

    if (!valid.value) return;

    await withLoading(async () => {
        try {
            await pmStore.inviteMember(email.value, props.projectId);

            if (configStore.state.config.unregisteredInviteEmailsEnabled) {
                notify.success('Invite sent!');
            } else {
                notify.success(
                    'An invitation will be sent to the email address if it belongs to a user on this satellite.',
                    'Invite sent!',
                );
            }

            email.value = '';
        } catch (error) {
            error.message = `Error inviting project member. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
            return;
        }

        analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_MEMBERS_INVITE_SENT);

        try {
            await pmStore.getProjectMembers(1, props.projectId);
        } catch (error) {
            error.message = `Unable to fetch project members. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
        }

        model.value = false;
    });
}
</script>
