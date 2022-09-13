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
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { Bucket } from '@/types/buckets';
import { RouteConfig } from '@/router';
import { MONTHS_NAMES } from '@/utils/constants/date';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { AnalyticsHttpApi } from '@/api/analytics';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import BucketDetailsOverview from '@/components/objects/BucketDetailsOverview.vue';

import ArrowRightIcon from '@/../static/images/common/arrowRight.svg';

// @vue/component
@Component({
    components: {
        ArrowRightIcon,
        BucketDetailsOverview,
    },
})
export default class BucketDetails extends Vue {
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook before initial render.
     * Checks if bucket name was passed as route param.
     */
    public async beforeMount(): Promise<void> {
        if (!this.$route.params.bucketName) {
            await this.redirectToBucketsPage();
        }
    }

    /**
     * Bucket from store found by router prop.
     */
    public get bucket(): Bucket {
        const data = this.$store.state.bucketUsageModule.page.buckets.find((bucket: Bucket) => bucket.name === this.$route.params.bucketName);

        if (!data) {
            this.redirectToBucketsPage();

            return new Bucket();
        }

        return data;
    }

    public get creationDate(): string {
        return `${this.bucket.since.getUTCDate()} ${MONTHS_NAMES[this.bucket.since.getUTCMonth()]} ${this.bucket.since.getUTCFullYear()}`;
    }

    public async redirectToBucketsPage(): Promise<void> {
        try {
            await this.$router.push({ name: RouteConfig.BucketsManagement.name });
        } catch (_) {
            return;
        }
    }

    /**
     * Holds on bucket click. Proceeds to file browser.
     */
    public openBucket(): void {
        this.$store.dispatch(OBJECTS_ACTIONS.SET_FILE_COMPONENT_BUCKET_NAME, this.bucket?.name);

        if (this.$route.params.backRoute === RouteConfig.BucketsManagement.name) {
            this.isNewObjectsFlow
                ? this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_OPEN_BUCKET_MODAL_SHOWN)
                : this.$router.push(RouteConfig.Buckets.with(RouteConfig.EncryptData).path);

            return;
        }

        this.analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
        this.$router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
    }

    /**
     * Returns objects flow status from store.
     */
    private get isNewObjectsFlow(): string {
        return this.$store.state.appStateModule.isNewObjectsFlow;
    }
}
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
