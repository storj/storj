// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="uc-area">
        <div class="uc-area__popup">
            <h1 class="uc-area__popup__title">Upload in progress</h1>
            <div class="uc-area__popup__container">
                <div class="uc-area__popup__container__header">
                    <WarningIcon />
                    <h2 class="uc-area__popup__container__header__question">Are you sure you want to leave?</h2>
                </div>
                <p class="uc-area__popup__container__msg">
                    Navigating to another page while uploading data may cancel your upload. Please confirm before
                    proceeding.
                </p>
            </div>
            <VButton
                width="100%"
                height="48px"
                label="Continue uploading"
                :on-press="closePopup"
            />
            <p class="uc-area__popup__link" @click="onLeaveClick">Cancel upload and leave</p>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { AnalyticsHttpApi } from '@/api/analytics';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import VButton from '@/components/common/VButton.vue';

import WarningIcon from '@/../static/images/objects/cancelWarning.svg';

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const router = useRouter();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Returns leave attempt's route path from store.
 */
const leaveRoute = computed((): string => {
    return bucketsStore.state.leaveRoute;
});

/**
 * Holds on leave click logic.
 */
function onLeaveClick(): void {
    analytics.pageVisit(leaveRoute.value);
    router.push(leaveRoute.value);
    closePopup();
}

/**
 * Close upload cancel info popup.
 */
function closePopup(): void {
    appStore.updateActiveModal(MODALS.uploadCancelPopup);
}
</script>

<style scoped lang="scss">
    .uc-area {
        position: fixed;
        inset: 0;
        z-index: 100;
        background: rgb(27 37 51 / 75%);
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: 'font_regular', sans-serif;

        &__popup {
            padding: 70px;
            border-radius: 8px;
            background-color: #fff;
            display: flex;
            flex-direction: column;
            align-items: center;

            &__title {
                width: 100%;
                text-align: left;
                font-family: 'font_bold', sans-serif;
                font-size: 23px;
                line-height: 49px;
                letter-spacing: -0.1007px;
                color: #252525;
                margin: 0 0 22px;
            }

            &__container {
                background: #f7f8fb;
                border-radius: 8px;
                margin-bottom: 22px;
                max-width: 465px;
                padding: 20px;

                &__header {
                    display: flex;
                    align-items: center;
                    margin-bottom: 10px;

                    &__question {
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                        line-height: 19px;
                        color: #1b2533;
                        margin: 0 0 0 10px;
                    }
                }
            }

            &__link {
                font-weight: 500;
                font-size: 16px;
                line-height: 21px;
                color: #0068dc;
                margin: 22px 0 0;
                cursor: pointer;
            }
        }
    }
</style>
