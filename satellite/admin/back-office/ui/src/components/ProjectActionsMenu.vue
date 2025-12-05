// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-menu activator="parent">
        <v-list class="pa-2">
            <v-list-item
                v-if="featureFlags.project.view && !isCurrentRouteViewProject" density="comfortable"
                link rounded="lg" base-color="info"
                @click="viewProject"
            >
                <v-list-item-title class="text-body-2 font-weight-medium">
                    View Project
                </v-list-item-title>
            </v-list-item>

            <v-list-item
                v-if="featureFlags.project.updatePlacement"
                density="comfortable" rounded="lg"
                link
                @click="emit('viewEntitlements', projectId)"
            >
                <v-list-item-title class="text-body-2 font-weight-medium">
                    View Entitlements
                </v-list-item-title>
            </v-list-item>

            <v-list-item
                v-if="hasUpdateProjectPerm"
                density="comfortable" rounded="lg"
                link
                @click="emit('update', projectId)"
            >
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Edit Project
                </v-list-item-title>
            </v-list-item>

            <v-list-item
                v-if="featureFlags.project.updateLimits"
                density="comfortable" link
                rounded="lg"
                @click="emit('updateLimits', projectId)"
            >
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Change Limits
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.bucket.create" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    New Bucket
                    <ProjectNewBucketDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item v-if="featureFlags.project.memberAdd" density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Add User
                    <ProjectAddUserDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item
                v-if="featureFlags.project.delete && active"
                density="comfortable" link
                rounded="lg" base-color="error"
                @click="emit('delete', projectId)"
            >
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Delete
                </v-list-item-title>
            </v-list-item>

            <v-list-item
                v-if="featureFlags.project.markPendingDeletion && active"
                density="comfortable"
                rounded="lg" link
                base-color="error"
                @click="emit('markPendingDeletion', projectId)"
            >
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Mark Pending Deletion
                </v-list-item-title>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script setup lang="ts">
import { VMenu, VList, VListItem, VListItemTitle } from 'vuetify/components';
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { FeatureFlags, User } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { ROUTES } from '@/router';

import ProjectNewBucketDialog from '@/components/ProjectNewBucketDialog.vue';
import ProjectAddUserDialog from '@/components/ProjectAddUserDialog.vue';

const appStore = useAppStore();
const router = useRouter();

const featureFlags = computed(() => appStore.state.settings.admin.features as FeatureFlags);

const hasUpdateProjectPerm = computed(() => {
    return featureFlags.value.project.updateInfo ||
      featureFlags.value.project.updatePlacement ||
      featureFlags.value.project.updateValueAttribution;
});

const props = defineProps<{
    projectId: string;
    active: boolean;
    owner: User;
}>();

const emit = defineEmits<{
    (e: 'updateLimits', projectId: string): void;
    (e: 'update', projectId: string): void;
    (e: 'viewEntitlements', projectId: string): void;
    (e: 'delete', projectId: string): void;
    (e: 'markPendingDeletion', projectId: string): void;
}>();

const isCurrentRouteViewProject = computed(() => {
    return router.currentRoute.value.name === ROUTES.AccountProject.name || router.currentRoute.value.name === ROUTES.ProjectDetail.name;
});

function viewProject() {
    if (router.currentRoute.value.name === ROUTES.Account.name) {
        router.push({
            name: ROUTES.AccountProject.name,
            params: { userID: props.owner.id, projectID: props.projectId },
        });
    }
}
</script>
