// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Project Management API Keys" />
        <PageSubtitleComponent subtitle="Interact with projects using API Keys" link="https://github.com/storj/storj/blob/main/satellite/console/consoleweb/consoleapi/apidocs.gen.md" />

        <v-col>
            <v-row class="mt-1 mb-3">
                <v-btn :prepend-icon="CirclePlus" @click="onCreateAPIKey">
                    New API Key
                </v-btn>
            </v-row>
        </v-col>

        <APIKeysTableComponent />
    </v-container>

    <NewApiKeyDialog v-model="dialog" />
</template>

<script setup lang="ts">
import { onBeforeMount, ref } from 'vue';
import {
    VContainer,
    VCol,
    VRow,
    VBtn,
} from 'vuetify/components';
import { CirclePlus } from 'lucide-vue-next';
import { useRouter } from 'vue-router';

import { useUsersStore } from '@/store/modules/usersStore';
import { ROUTES } from '@/router';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import APIKeysTableComponent from '@/components/RestApiKeysTableComponent.vue';
import NewApiKeyDialog from '@/components/dialogs/NewRestApiKeyDialog.vue';

const router = useRouter();
const userStore = useUsersStore();

const dialog = ref<boolean>(false);

/**
 * Starts create access grant flow if user's free trial is not expired.
 */
function onCreateAPIKey(): void {
    dialog.value = true;
}

onBeforeMount(() => {
    if (!userStore.state.user.hasPaidPrivileges) {
        router.replace(ROUTES.Projects.path);
    }
});
</script>
