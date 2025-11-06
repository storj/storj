// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="420px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="UserPlus" :size="18" />
                    </v-sheet>
                </template>

                <v-card-title class="font-weight-bold">Add Member</v-card-title>

                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-form v-model="valid" class="pa-6 pb-4" @submit.prevent="onPrimaryClick">
                <v-row>
                    <v-col cols="12">
                        <p class="mb-5">Invite a team member to join you in this project.</p>
                        <v-alert
                            variant="tonal"
                            color="info"
                            text="All team members should use the same passphrase to access the same data."
                            rounded="lg"
                            density="comfortable"
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-text-field
                            v-model="email"
                            variant="outlined"
                            :rules="emailRules"
                            maxlength="72"
                            label="Enter e-mail"
                            placeholder="Enter e-mail here"
                            hint="Members will have read, write, and delete permissions."
                            required
                            autofocus
                            class="my-2"
                        />
                    </v-col>
                </v-row>
            </v-form>

            <v-divider />

            <v-card-actions class="pa-6">
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
                            @click="onPrimaryClick"
                        >
                            Send Invite
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
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
    VSheet,
} from 'vuetify/components';
import { UserPlus, X } from 'lucide-vue-next';

import { EmailRule, RequiredRule, ValidationRule } from '@/types/common';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

const props = defineProps<{
    projectId: string;
}>();

const model = defineModel<boolean>({ required: true });

const analyticsStore = useAnalyticsStore();
const pmStore = useProjectMembersStore();
const configStore = useConfigStore();
const projectStore = useProjectsStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const valid = ref<boolean>(false);
const email = ref<string>('');

const emailRules: ValidationRule<string>[] = [
    RequiredRule,
    (value) => EmailRule(value, true),
];

/**
 * Handles primary button click.
 */
function onPrimaryClick(): void {
    if (!valid.value) return;

    withLoading(async () => {
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
            notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
            return;
        }

        analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_MEMBERS_INVITE_SENT, { project_id: projectStore.state.selectedProject.id });

        try {
            await pmStore.getProjectMembers(1, props.projectId);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
        }

        model.value = false;
    });
}
</script>
