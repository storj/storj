// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        scrollable
        max-width="400px"
        transition="fade-transition"
    >
        <v-card>
            <v-card-item class="pa-6">
                <v-card-title class="font-weight-bold">Upgrade Account</v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item>
                <v-card-text class="pa-0">
                    This action requires a paid plan. The project owner is currently on a free trial and must upgrade to proceed.
                </v-card-text>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            v-if="isOwner"
                            color="primary"
                            variant="flat"
                            block
                            @click="toggleUpgradeFlow"
                        >
                            Upgrade Account
                        </v-btn>
                        <v-btn
                            v-else
                            color="primary"
                            variant="flat"
                            block
                            @click="model = false"
                        >
                            Close
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCardText,
    VCol,
    VDialog,
    VDivider,
    VRow,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';

const appStore = useAppStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();

const model = defineModel<boolean>({ required: true });

const isOwner = computed<boolean>(() => projectsStore.state.selectedProject.ownerId === usersStore.state.user.id);

/**
 * Toggles upgrade account flow visibility.
 */
function toggleUpgradeFlow(): void {
    model.value = false;
    appStore.toggleUpgradeFlow(true);
}
</script>
