// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Team Members" />
        <PageSubtitleComponent
            subtitle="Invite people and manage the team of this project."
            link="https://docs.storj.io/support/users"
        />

        <v-col>
            <v-row class="mt-2 mb-4">
                <v-btn @click="isAddMemberDialogShown = true">
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

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import TeamTableComponent from '@/components/TeamTableComponent.vue';
import AddTeamMemberDialog from '@/components/dialogs/AddTeamMemberDialog.vue';
import IconNew from '@/components/icons/IconNew.vue';

const projectsStore = useProjectsStore();

const isAddMemberDialogShown = ref<boolean>(false);

const selectedProjectID = computed((): string => projectsStore.state.selectedProject.id);
</script>
