// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <trial-expiration-banner v-if="isTrialExpirationBanner" :expired="isExpired" />

        <PageTitleComponent title="Team Members" />
        <PageSubtitleComponent
            subtitle="Invite people and manage the team of this project."
            link="https://docs.storj.io/support/users"
        />

        <v-col>
            <v-row class="mt-2 mb-4">
                <v-btn @click="onAddMember">
                    <IconNew class="mr-2" size="16" bold />
                    Add Members
                </v-btn>
            </v-row>
        </v-col>

        <TeamTableComponent />
    </v-container>

    <add-team-member-dialog v-model="isAddMemberDialogShown" :project-id="selectedProjectID" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VContainer, VCol, VRow, VBtn } from 'vuetify/components';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useTrialCheck } from '@/composables/useTrialCheck';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import TeamTableComponent from '@/components/TeamTableComponent.vue';
import AddTeamMemberDialog from '@/components/dialogs/AddTeamMemberDialog.vue';
import IconNew from '@/components/icons/IconNew.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';

const projectsStore = useProjectsStore();

const { isTrialExpirationBanner, isExpired, withTrialCheck } = useTrialCheck();

const isAddMemberDialogShown = ref<boolean>(false);

const selectedProjectID = computed((): string => projectsStore.state.selectedProject.id);

/**
 * Starts create bucket flow if user's free trial is not expired.
 */
function onAddMember(): void {
    withTrialCheck(() => {
        isAddMemberDialogShown.value = true;
    });
}
</script>
