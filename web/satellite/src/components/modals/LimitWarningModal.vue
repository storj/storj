// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-modal :on-close="onClose">
        <template #content>
            <div class="modal">
                <div class="modal__icon" :class="isHundred ? 'critical' : 'warning'">
                    <Icon />
                </div>
                <h1 class="modal__title">
                    <template v-if="isHundred">
                        Urgent! You've reached the {{ limitTypes }} limit{{ limitTypes.includes(' ') ? 's' : '' }} for your project.
                    </template>
                    <template v-else>
                        80% {{ limitTypes.charAt(0).toUpperCase() + limitTypes.slice(1) }} used
                    </template>
                </h1>
                <p class="modal__info">
                    <template v-if="!isPaidTier">
                        To get more {{ limitTypes }} limit{{ limitTypes.includes(' ') ? 's' : '' }}, upgrade to a Pro Account.
                        You will still get {{ bytesToBase10String(limits.storageUsed) }} free storage and egress per month, and only pay what you use beyond that.
                    </template>
                    <template v-else-if="isCustom">
                        You can increase your limits in the Project Settings page.
                    </template>
                    <template v-else>
                        To get higher limits, please contact support.
                    </template>
                </p>
                <div class="modal__buttons">
                    <VButton
                        label="Cancel"
                        height="40px"
                        width="48%"
                        font-size="13px"
                        class="modal__buttons__button"
                        :is-white="true"
                        :on-press="onClose"
                    />
                    <VButton
                        :label="!isPaidTier ? 'Upgrade' : isCustom ? 'Edit Limits' : 'Request Higher Limit'"
                        height="40px"
                        width="48%"
                        font-size="13px"
                        class="modal__buttons__button primary"
                        :on-press="onPrimaryClick"
                        :link="(isPaidTier && !isCustom) ? requestURL : undefined"
                        :is-white-blue="true"
                    />
                </div>
            </div>
        </template>
    </v-modal>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { ProjectLimits, LimitThreshold, LimitThresholdsReached } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { bytesToBase10String, humanizeArray } from '@/utils/strings';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { RouteConfig } from '@/types/router';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

import Icon from '@/../static/images/notifications/info.svg';

const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();
const analyticsStore = useAnalyticsStore();

const router = useRouter();

const props = defineProps<{
    reachedThresholds: LimitThresholdsReached;
    threshold: LimitThreshold,
    onUpgrade: () => void;
    onClose: () => void
}>();

/**
 * Returns whether the threshold represents 100% usage.
 */
const isHundred = computed((): boolean => props.threshold.toLowerCase().includes('hundred'));

/**
 * Returns whether the usage limit threshold is for a custom limit.
 */
const isCustom = computed((): boolean => props.threshold.toLowerCase().includes('custom'));

/**
 * Returns a string representing the usage types that have reached this limit threshold.
 */
const limitTypes = computed((): string => {
    return humanizeArray(props.reachedThresholds[props.threshold]).toLowerCase();
});

/**
 * Returns current limits from store.
 */
const limits = computed((): ProjectLimits => {
    return projectsStore.state.currentLimits;
});

/**
 * Returns whether user is in the paid tier.
 */
const isPaidTier = computed((): boolean => {
    return usersStore.state.user.paidTier;
});

/**
 * Returns the URL for the general request page from the store.
 */
const requestURL = computed((): string => {
    return configStore.state.config.generalRequestURL;
});

/**
 * Handles primary button click.
 */
function onPrimaryClick(): void {
    if (!isPaidTier.value) {
        props.onUpgrade();
        return;
    }
    if (isCustom.value) {
        analyticsStore.pageVisit(RouteConfig.EditProjectDetails.path);
        router.push(RouteConfig.EditProjectDetails.path);
        props.onClose();
    }
}
</script>

<style scoped lang="scss">
.modal {
    width: 500px;
    max-width: calc(100vw - 48px);
    padding: 32px;
    box-sizing: border-box;
    font-family: 'font_regular', sans-serif;
    text-align: left;

    &__icon {
        width: 64px;
        height: 64px;
        margin-bottom: 24px;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 24px;

        :deep(svg) {
            width: 46px;
            height: 46px;
        }

        &.critical {
            background-color: var(--c-pink-1);

            :deep(path) {
                fill: var(--c-pink-4);
            }
        }

        &.warning {
            background-color: var(--c-yellow-1);

            :deep(path) {
                fill: var(--c-yellow-5);
            }
        }
    }

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 28px;
        line-height: 36px;
        letter-spacing: -0.02em;
        color: #000;
        margin-bottom: 8px;
    }

    &__info {
        font-family: 'font_regular', sans-serif;
        font-size: 16px;
        line-height: 24px;
        color: #000;
        margin-bottom: 16px;
    }

    &__buttons {
        display: flex;
        flex-direction: row;
        justify-content: space-between;

        &__button {
            padding: 16px;
            box-sizing: border-box;
            letter-spacing: -0.02em;

            &.primary {
                margin-left: 8px;
            }
        }
    }
}
</style>
