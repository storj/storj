// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isBannerShowing" class="notification-wrap">
        <div class="notification-wrap__left">
            <SunnyIcon class="notification-wrap__left__icon" />
            <p>
                Ready to upgrade? Upload up to 75TB and pay what you use only, no minimum.
                {{ limits.bandwidthLimit | bytesToBase10String }} free included.
            </p>
        </div>
        <div class="notification-wrap__right">
            <a @click="openBanner">Upgrade Now</a>
            <CloseIcon class="notification-wrap__right__close" @click="onCloseClick" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { ProjectLimits } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';

import SunnyIcon from '@/../static/images/notifications/sunnyicon.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';

const props = defineProps<{
    openAddPMModal: () => void,
}>();

const projectsStore = useProjectsStore();
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const isBannerShowing = ref<boolean>(true);

/**
 * Closes notification.
 */
function onCloseClick(): void {
    isBannerShowing.value = false;
}

/**
 * Returns current limits from store.
 */
const limits = computed((): ProjectLimits => {
    return projectsStore.state.currentLimits;
});

/**
 * Send analytics event to segment when Upgrade Account banner is clicked.
 */
async function openBanner(): Promise<void> {
    props.openAddPMModal();
    await analytics.eventTriggered(AnalyticsEvent.UPGRADE_BANNER_CLICKED);
}
</script>

<style scoped lang="scss">
.notification-wrap {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1.375rem;
    font-family: 'font_regular', sans-serif;
    font-size: 1rem;
    background-color: var(--c-white);
    border: 1px solid var(--c-blue-2);
    border-radius: 10px;
    box-shadow: 0 7px 20px rgba(0 0 0 / 15%);

    &__left {
        display: flex;
        align-items: center;

        &__icon {
            flex-shrink: 0;
            margin-right: 1.375rem;
        }
    }

    &__right {
        display: flex;
        align-items: center;
        flex-shrink: 0;
        margin-left: 16px;

        & a {
            color: var(--c-black);
            text-decoration: underline !important;
        }

        &__close {
            margin-left: 16px;
            cursor: pointer;
        }
    }
}
</style>
