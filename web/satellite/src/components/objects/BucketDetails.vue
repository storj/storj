// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bucket-details">
        <div class="bucket-details__header">
            <div class="bucket-details__header__left-area">
                <p class="bucket-details__header__left-area__link" @click.stop="redirectToBucketsPage">Buckets</p>
                <arrow-right-icon />
                <p class="bold">{{ bucket.name }}</p>
                <arrow-right-icon />
                <p>Bucket Details</p>
            </div>
            <div class="bucket-details__header__right-area">
                <p>{{ bucket.name }} created at {{ creationDate }}</p>
                <bucket-settings-nav :bucket-name="bucket" />
            </div>
        </div>
        <bucket-details-overview class="bucket-details__table" :bucket="bucket" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import BucketDetailsOverview from "@/components/objects/BucketDetailsOverview.vue";
import BucketSettingsNav from "@/components/objects/BucketSettingsNav.vue";
import ArrowRightIcon from '@/../static/images/common/arrowRight.svg'

import { Bucket } from "@/types/buckets";
import { RouteConfig } from "@/router";
import { MONTHS_NAMES } from "@/utils/constants/date";

// @vue/component
@Component({
    components: {
        ArrowRightIcon,
        BucketDetailsOverview,
        BucketSettingsNav,
    },
})
export default class BucketDetails extends Vue {
    /**
     * Bucket from store found by router prop.
     */
    public get bucket(): Bucket | undefined {
        const data = this.$store.state.bucketUsageModule.page.buckets.find((bucket: Bucket) => bucket.name === this.$route.params.bucketName);

        if (!data) {
            this.redirectToBucketsPage();

            return new Bucket();
        }

        return data;
    }

    public get creationDate(): string {
        return !this.bucket ?
            '' :
            `${this.bucket.since.getUTCDate()} ${MONTHS_NAMES[this.bucket.since.getUTCMonth()]} ${this.bucket.since.getUTCFullYear()}`;
    }

    public redirectToBucketsPage(): void {
        this.$router.push({ name: RouteConfig.Buckets.with(RouteConfig.BucketsManagement).name });
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
