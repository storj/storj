// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bucket-details">
        <div class="bucket-details__header">
            <div class="bucket-details__header__left-area">
                <p class="bucket-details__header__left-area link" @click.stop="redirectToBucketsPage">Buckets</p>
                <arrow-right-icon />
                <p class="bold link" @click.stop="openBucket">{{ bucket.name }}</p>
                <arrow-right-icon />
                <p>Bucket Details</p>
            </div>
            <div class="bucket-details__header__right-area">
                <p>{{ bucket.name }} created at {{ creationDate }}</p>
            </div>
        </div>
        <bucket-details-overview class="bucket-details__table" :bucket="bucket" />
        <VOverallLoader v-if="isLoading" />
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, reactive, ref } from 'vue';

import { Bucket } from '@/types/buckets';
import { RouteConfig } from '@/router';
import { MONTHS_NAMES } from '@/utils/constants/date';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { AnalyticsHttpApi } from '@/api/analytics';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';

import BucketDetailsOverview from '@/components/objects/BucketDetailsOverview.vue';
import VOverallLoader from '@/components/common/VOverallLoader.vue';

import ArrowRightIcon from '@/../static/images/common/arrowRight.svg';

const appStore = useAppStore();
const store = useStore();
const notify = useNotify();
const nativeRouter = useRouter();
const router = reactive(nativeRouter);

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const isLoading = ref<boolean>(false);

/**
 * Returns condition if user has to be prompt for passphrase from store.
 */
const promptForPassphrase = computed((): boolean => {
    return store.state.objectsModule.promptForPassphrase;
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => {
    return store.state.objectsModule.gatewayCredentials;
});

/**
 * Bucket from store found by router prop.
 */
const bucket = computed((): Bucket => {
    const data = store.state.bucketUsageModule.page.buckets.find((bucket: Bucket) => bucket.name === router.currentRoute.params.bucketName);

    if (!data) {
        redirectToBucketsPage();

        return new Bucket();
    }

    return data;
});

const creationDate = computed((): string => {
    return `${bucket.value.since.getUTCDate()} ${MONTHS_NAMES[bucket.value.since.getUTCMonth()]} ${bucket.value.since.getUTCFullYear()}`;
});

function redirectToBucketsPage(): void {
    router.push({ name: RouteConfig.BucketsManagement.name }).catch(() => {return;});
}

/**
 * Holds on bucket click. Proceeds to file browser.
 */
async function openBucket(): Promise<void> {
    await store.dispatch(OBJECTS_ACTIONS.SET_FILE_COMPONENT_BUCKET_NAME, bucket.value.name);

    if (router.currentRoute.params.backRoute === RouteConfig.UploadFileChildren.name || !promptForPassphrase.value) {
        if (!edgeCredentials.value.accessKeyId) {
            isLoading.value = true;

            try {
                await store.dispatch(OBJECTS_ACTIONS.SET_S3_CLIENT);
                isLoading.value = false;
            } catch (error) {
                await notify.error(error.message, AnalyticsErrorEventSource.BUCKET_DETAILS_PAGE);
                isLoading.value = false;
                return;
            }
        }

        analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
        router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);

        return;
    }

    appStore.updateActiveModal(MODALS.openBucket);
}

/**
 * Lifecycle hook before initial render.
 * Checks if bucket name was passed as route param.
 */
onBeforeMount((): void => {
    if (!router.currentRoute.params.bucketName) {
        redirectToBucketsPage();
    }
});
</script>

<style lang="scss" scoped>
.bucket-details {
    width: 100%;

    &__header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        font-family: 'font_regular', sans-serif;
        color: #1b2533;

        &__left-area {
            display: flex;
            align-items: center;
            justify-content: flex-start;

            svg {
                margin: 0 15px;
            }

            .bold {
                font-family: 'font_bold', sans-serif;
            }

            .link {
                cursor: pointer;
            }
        }

        &__right-area {
            display: flex;
            align-items: center;
            justify-content: flex-end;

            p {
                opacity: 0.2;
                margin-right: 17px;
            }
        }
    }

    &__table {
        margin-top: 40px;
    }
}
</style>
