// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Team" />
        <PageSubtitleComponent subtitle="Invite people and manage the team of this project." link="https://docs.storj.io/dcs/users" />

        <v-col>
            <v-row class="mt-2 mb-4">
                <v-btn>
                    <svg width="16" height="16" class="mr-2" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M10 1C14.9706 1 19 5.02944 19 10C19 14.9706 14.9706 19 10 19C5.02944 19 1 14.9706 1 10C1 5.02944 5.02944 1 10 1ZM10 2.65C5.94071 2.65 2.65 5.94071 2.65 10C2.65 14.0593 5.94071 17.35 10 17.35C14.0593 17.35 17.35 14.0593 17.35 10C17.35 5.94071 14.0593 2.65 10 2.65ZM10.7496 6.8989L10.7499 6.91218L10.7499 9.223H12.9926C13.4529 9.223 13.8302 9.58799 13.8456 10.048C13.8602 10.4887 13.5148 10.8579 13.0741 10.8726L13.0608 10.8729L10.7499 10.873L10.75 13.171C10.75 13.6266 10.3806 13.996 9.925 13.996C9.48048 13.996 9.11807 13.6444 9.10066 13.2042L9.1 13.171L9.09985 10.873H6.802C6.34637 10.873 5.977 10.5036 5.977 10.048C5.977 9.60348 6.32857 9.24107 6.76882 9.22366L6.802 9.223H9.09985L9.1 6.98036C9.1 6.5201 9.46499 6.14276 9.925 6.12745C10.3657 6.11279 10.7349 6.45818 10.7496 6.8989Z" fill="currentColor" />
                    </svg>
                    Add Members

                    <v-dialog
                        v-model="dialog"
                        activator="parent"
                        width="auto"
                        min-width="400px"
                        transition="fade-transition"
                    >
                        <v-card rounded="xlg">
                            <v-sheet>
                                <v-card-item class="pl-7 py-4">
                                    <template #prepend>
                                        <v-card-title class="font-weight-bold">
                                            <!-- <v-icon>
                                                <img src="../assets/icon-team.svg" alt="Team">
                                            </v-icon> -->
                                            Add Members
                                        </v-card-title>
                                    </template>

                                    <!-- <v-btn
                                        class="text-none text-subtitle-1"
                                        color="#5865f2"
                                        size="small"
                                        variant="flat"
                                    >
                                        + Add More
                                    </v-btn> -->

                                    <template #append>
                                        <v-btn
                                            icon="$close"
                                            variant="text"
                                            size="small"
                                            color="default"
                                            @click="dialog = false"
                                        />
                                    </template>
                                </v-card-item>
                            </v-sheet>

                            <v-divider />

                            <v-form v-model="valid" class="pa-7 pb-4" @submit.prevent="onAddUsersClick">
                                <v-row>
                                    <v-col>
                                        <p>Invite team members to join you in this project.</p>
                                    <!-- <v-divider class="my-6"></v-divider> -->
                                    <!-- <p>Use only lowercase letters and numbers, no spaces.</p> -->
                                    <!-- <v-chip prepend-icon="mdi-information" color="info" rounded="xl">
                                        Members will have read & write permissions.
                                    </v-chip> -->
                                    </v-col>
                                </v-row>

                                <v-row>
                                    <v-col
                                        cols="12"
                                    >
                                        <v-text-field
                                            v-model="email"
                                            variant="outlined"
                                            :rules="emailRules"
                                            label="Enter e-mail"
                                            hint="Members will have read & write permissions."
                                            required
                                            autofocus
                                            class="mt-2"
                                        />
                                    </v-col>
                                </v-row>

                                <!-- <v-row>
                                    <v-col>
                                        <v-btn variant="text" class="mb-4">+ Add More</v-btn>
                                    </v-col>
                                </v-row> -->
                            </v-form>

                            <v-divider />

                            <v-card-actions class="pa-7">
                                <v-row>
                                    <v-col>
                                        <v-btn variant="outlined" color="default" block @click="dialog = false">Cancel</v-btn>
                                    </v-col>
                                    <v-col>
                                        <v-btn color="primary" variant="flat" block :loading="isLoading" @click="onAddUsersClick">Send Invite</v-btn>
                                    </v-col>
                                </v-row>
                            </v-card-actions>
                        </v-card>
                    </v-dialog>
                </v-btn>
            </v-row>
        </v-col>

        <TeamTableComponent />
    </v-container>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
    VContainer,
    VCol,
    VRow,
    VBtn,
    VDialog,
    VCard,
    VSheet,
    VCardItem,
    VCardTitle,
    VDivider,
    VForm,
    VCardActions,
    VTextField,
} from 'vuetify/components';

import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@poc/components/PageSubtitleComponent.vue';
import TeamTableComponent from '@poc/components/TeamTableComponent.vue';

const analyticsStore = useAnalyticsStore();
const pmStore = useProjectMembersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const isLoading = ref<boolean>(false);
const dialog = ref<boolean>(false);
const valid = ref<boolean>(false);
const email = ref<string>('');

const emailRules = [
    (value: string): string | boolean => (!!value || 'E-mail is requred.'),
    (value: string): string | boolean => ((/.+@.+\..+/.test(value)) || 'E-mail must be valid.'),
];

const selectedProjectID = computed((): string => projectsStore.state.selectedProject.id);

/**
 * Tries to add users related to entered emails list to current project.
 */
async function onAddUsersClick(): Promise<void> {
    if (isLoading.value || !valid.value) return;

    isLoading.value = true;

    try {
        await pmStore.inviteMembers([email.value], selectedProjectID.value);
        notify.notify('Invites sent!');
        email.value = '';
    } catch (error) {
        error.message = `Error adding project members. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
        isLoading.value = false;
        return;
    }

    analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_MEMBERS_INVITE_SENT);

    try {
        await pmStore.getProjectMembers(1, selectedProjectID.value);
    } catch (error) {
        error.message = `Unable to fetch project members. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
    }

    dialog.value = false;
    isLoading.value = false;
}

onMounted(() => {
    pmStore.getProjectMembers(1, selectedProjectID.value);
});
</script>
