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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { Bucket } from '@/types/buckets';
import { RouteConfig } from '@/router';
import { MONTHS_NAMES } from '@/utils/constants/date';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { AnalyticsHttpApi } from '@/api/analytics';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import BucketDetailsOverview from '@/components/objects/BucketDetailsOverview.vue';
import VOverallLoader from '@/components/common/VOverallLoader.vue';

import ArrowRightIcon from '@/../static/images/common/arrowRight.svg';

// @vue/component
@Component({
    components: {
        VOverallLoader,
        ArrowRightIcon,
        BucketDetailsOverview,
    },
})
export default class BucketDetails extends Vue {
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public isLoading = false;

    /**
     * Lifecycle hook before initial render.
     * Checks if bucket name was passed as route param.
     */
    public beforeMount(): void {
        if (!this.$route.params.bucketName) {
            this.redirectToBucketsPage();
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

    public redirectToBucketsPage(): void {
        try {
            this.$router.push({ name: RouteConfig.BucketsManagement.name });
        } catch (_) {
            return;
        }
    }

    /**
     * Holds on bucket click. Proceeds to file browser.
     */
    public async openBucket(): Promise<void> {
        await this.$store.dispatch(OBJECTS_ACTIONS.SET_FILE_COMPONENT_BUCKET_NAME, this.bucket?.name);

        if (this.$route.params.backRoute === RouteConfig.UploadFileChildren.name || !this.promptForPassphrase) {
            if (!this.edgeCredentials.accessKeyId) {
                this.isLoading = true;

                try {
                    await this.$store.dispatch(OBJECTS_ACTIONS.SET_S3_CLIENT);
                    this.isLoading = false;
                } catch (error) {
                    await this.$notify.error(error.message, AnalyticsErrorEventSource.BUCKET_DETAILS_PAGE);
                    this.isLoading = false;
                    return;
                }
            }

            this.analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
            this.$router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);

            return;
        }

        this.$store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.openBucket);
    }

    /**
     * Returns condition if user has to be prompt for passphrase from store.
     */
    private get promptForPassphrase(): boolean {
        return this.$store.state.objectsModule.promptForPassphrase;
    }

    /**
     * Returns edge credentials from store.
     */
    private get edgeCredentials(): EdgeCredentials {
        return this.$store.state.objectsModule.gatewayCredentials;
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
