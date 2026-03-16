// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card title="Email Notifications" class="pa-2">
        <v-card-subtitle>
            Storage Limit Notifications: <b>{{ project.storageLimitNotificationsEnabled ? 'Enabled' : 'Disabled' }}</b><br>
            Egress Limit Notifications: <b>{{ project.egressLimitNotificationsEnabled ? 'Enabled' : 'Disabled' }}</b>
        </v-card-subtitle>
        <v-card-text>
            <v-btn
                variant="outlined"
                color="default"
                rounded="md"
                :append-icon="ArrowRight"
                @click="isLimitNotificationsDialogShown = true"
            >
                Update
            </v-btn>
        </v-card-text>
    </v-card>
    <project-limit-notifications-dialog v-model="isLimitNotificationsDialogShown" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VCard,
    VCardText,
    VCardSubtitle,
    VBtn,
} from 'vuetify/components';
import { ArrowRight } from 'lucide-vue-next';

import { Project } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';

import ProjectLimitNotificationsDialog from '@/components/dialogs/ProjectLimitNotificationsDialog.vue';

const projectsStore = useProjectsStore();

const isLimitNotificationsDialogShown = ref<boolean>(false);

const project = computed<Project>(() => projectsStore.state.selectedProject);
</script>
