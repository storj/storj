// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isBannerShowing" class="notification-wrap">
        <SunnyIcon class="notification-wrap__icon" />
        <div class="notification-wrap__text">
            <p>
                Ready to upgrade? Upload up to 75TB and pay what you use only, no minimum.
                {{ formattedStorageLimit }} free included.
            </p>
            <a @click="openBanner">Upgrade Now</a>
        </div>
        <CloseIcon class="notification-wrap__close" @click="onCloseClick" />
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { useUsersStore } from '@/store/modules/usersStore';
import { Size } from '@/utils/bytesSize';

import SunnyIcon from '@/../static/images/notifications/sunnyicon.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';

const props = defineProps<{
    openAddPMModal: () => void,
}>();

const usersStore = useUsersStore();
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const isBannerShowing = ref<boolean>(true);

/**
 * Closes notification.
 */
function onCloseClick(): void {
    isBannerShowing.value = false;
}

/**
 * Returns the user's project storage limit from the store formatted as a size string.
 */
const formattedStorageLimit = computed((): string => {
    return Size.toBase10String(usersStore.state.user.projectStorageLimit);
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
    gap: 16px;
    padding: 16px;
    font-family: 'font_regular', sans-serif;
    font-size: 1rem;
    background-color: var(--c-white);
    border: 1px solid var(--c-blue-2);
    border-radius: 10px;
    box-shadow: 0 7px 20px rgba(0 0 0 / 15%);

    &__icon {
        flex-shrink: 0;
    }

    &__text {
        display: flex;
        align-items: center;
        gap: 16px;
        flex-grow: 1;

        & a {
            color: var(--c-black);
            text-decoration: underline !important;
            white-space: nowrap;
        }

        @media screen and (width <= 500px) {
            flex-direction: column;
            align-items: flex-start;
        }
    }

    &__close {
        flex-shrink: 0;
        cursor: pointer;
    }

    @media screen and (width <= 500px) {
        align-items: flex-start;
    }
}
</style>
