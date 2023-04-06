// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-banner
        severity="warning"
        :dashboard-ref="dashboardRef"
        :on-close="onClose"
    >
        <template #text>
            <p><span class="bold">{{ bannerTextData.title }}</span> <span class="medium"> {{ bannerTextData.body }}</span></p>
            <p v-if="!isPaidTier" class="link" @click.stop.self="onUpgradeClicked">Upgrade Now</p>
            <a
                v-else
                :href="projectLimitsIncreaseRequestURL"
                class="link"
                target="_blank"
                rel="noopener noreferrer"
            >Request Limit Increase</a>
        </template>
    </v-banner>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { MetaUtils } from '@/utils/meta';
import { useStore } from '@/utils/hooks';
import { LocalData } from '@/utils/localData';
import { useUsersStore } from '@/store/modules/usersStore';

import VBanner from '@/components/common/VBanner.vue';

const usersStore = useUsersStore();
const store = useStore();

const props = defineProps<{
    dashboardRef: HTMLElement;
    onUpgradeClicked: () => void;
}>();

const bannerTextData = computed((): { title: string, body: string } => {
    if (isPaidTier.value) {
        return {
            title: 'Reached project limits.',
            body: ` You have used ${projectsCount.value} of your ${projectLimit.value} available projects.`,
        };
    }
    return {
        title: 'Need more projects?',
        body: 'Upgrade to a Pro Account and use up to three projects immediately.',
    };
});

const projectLimitsIncreaseRequestURL = computed((): string => {
    return MetaUtils.getMetaContent('project-limits-increase-request-url');
});

/**
 * Returns whether user is in paid tier.
 */
const isPaidTier = computed((): boolean => {
    return usersStore.state.user.paidTier;
});

/**
 * Returns user's projects count.
 */
const projectsCount = computed((): number => {
    return store.getters.projectsCount(usersStore.state.user.id);
});

/**
 * Returns project limit from store.
 */
const projectLimit = computed((): number => {
    const projectLimit: number = usersStore.state.user.projectLimit;
    if (projectLimit < projectsCount.value) return projectsCount.value;

    return projectLimit;
});

function onClose() {
    LocalData.setProjectLimitBannerHidden();
}
</script>
