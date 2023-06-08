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
import { computed, onBeforeMount, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Bucket } from '@/types/buckets';
import { RouteConfig } from '@/types/router';
import { MONTHS_NAMES } from '@/utils/constants/date';
import { AnalyticsHttpApi } from '@/api/analytics';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import BucketDetailsOverview from '@/components/objects/BucketDetailsOverview.vue';
import VOverallLoader from '@/components/common/VOverallLoader.vue';

import ArrowRightIcon from '@/../static/images/common/arrowRight.svg';

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();
const route = useRoute();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const isLoading = ref<boolean>(false);

/**
 * Returns condition if user has to be prompt for passphrase from store.
 */
const promptForPassphrase = computed((): boolean => {
    return bucketsStore.state.promptForPassphrase;
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => {
    return bucketsStore.state.edgeCredentials;
});

/**
 * Bucket from store found by router prop.
 */
const bucket = computed((): Bucket => {
    if (!projectsStore.state.selectedProject.id) return new Bucket();

    const data = bucketsStore.state.page.buckets.find(
        (bucket: Bucket) => bucket.name === route.query.bucketName,
    );

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
    bucketsStore.setFileComponentBucketName(bucket.value.name);

    if (route.query.backRoute === RouteConfig.UploadFileChildren.name || !promptForPassphrase.value) {
        if (!edgeCredentials.value.accessKeyId) {
            isLoading.value = true;

            try {
                await bucketsStore.setS3Client(projectsStore.state.selectedProject.id);
                isLoading.value = false;
            } catch (error) {
                notify.error(error.message, AnalyticsErrorEventSource.BUCKET_DETAILS_PAGE);
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
    if (!route.query.bucketName) {
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
