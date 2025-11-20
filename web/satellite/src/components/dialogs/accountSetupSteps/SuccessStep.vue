// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height" fluid>
        <v-row justify="center" align="center">
            <v-col class="text-center py-10">
                <component :is="CircleCheckBig" :size="40" />

                <p class="text-overline mt-4 mb-2">
                    Account Complete
                </p>
                <h2 class="mb-3">You are now ready to use {{ configStore.brandName }}</h2>
                <p class="mb-2">Create your first bucket, and start uploading files.</p>
                <p>Let us know if you need any help getting started!</p>
                <v-btn
                    id="continue-btn"
                    class="mt-7"
                    size="large"
                    :append-icon="ChevronRight"
                    :loading="loading"
                    @click="finishSetup()"
                >
                    Continue
                </v-btn>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCol, VContainer, VRow } from 'vuetify/components';
import { CircleCheckBig, ChevronRight } from 'lucide-vue-next';
import { nextTick } from 'vue';
import { useRouter } from 'vue-router';

import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { ONBOARDING_STEPPER_STEPS } from '@/types/users';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { ROUTES } from '@/router';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';

const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const userStore = useUsersStore();
const configStore = useConfigStore();

const router = useRouter();

defineProps<{
    loading: boolean,
}>();

const emit = defineEmits<{
    finish: [];
}>();

async function finishSetup() {
    const projects = projectsStore.state.projects;
    if (!projects.length) {
        await projectsStore.createDefaultProject(userStore.state.user.id);
    }
    projectsStore.selectProject(projects[0].id);

    analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
    await userStore.updateSettings({ onboardingStep: ONBOARDING_STEPPER_STEPS[0] });
    await userStore.getUser();

    await nextTick();
    await router.push({
        name: ROUTES.Dashboard.name,
        params: { id: projectsStore.state.selectedProject.urlId },
    });

    emit('finish');
}

defineExpose({
    setup: finishSetup,
});
</script>
