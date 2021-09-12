// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <section class="project-summary">
        <h1 class="project-summary__title">Details</h1>
        <VLoader v-if="isDataFetching"/>
        <div v-else class="project-summary__items">
            <SummaryItem
                class="right-indent"
                background-color="#f5f6fa"
                title-color="#1b2533"
                value-color="#000"
                title="Users"
                :value="teamSize"
            />
            <SummaryItem
                class="right-indent"
                background-color="#f5f6fa"
                title-color="#1b2533"
                value-color="#000"
                title="Access Grants"
                :value="accessGrantsAmount"
            />
            <SummaryItem
                class="right-indent"
                background-color="#b1c1d9"
                title-color="#1b2533"
                value-color="#000"
                title="Buckets"
                :value="bucketsAmount"
            />
            <SummaryItem
                background-color="#0068DC"
                title-color="#fff"
                value-color="#fff"
                title="Estimated Charges"
                :value="estimatedCharges"
                :is-money="true"
            />
        </div>
    </section>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VLoader from '@/components/common/VLoader.vue';
import SummaryItem from '@/components/project/summary/SummaryItem.vue';

@Component({
    components: {
        SummaryItem,
        VLoader,
    },
})
export default class ProjectSummary extends Vue {
    @Prop({ default: true })
    public readonly isDataFetching: boolean;

    /**
     * teamSize returns project members amount for selected project.
     */
    public get teamSize(): number {
        return this.$store.state.projectMembersModule.page.totalCount;
    }

    /**
     * accessGrantsAmount returns access grants' amount for selected project.
     */
    public get accessGrantsAmount(): number {
        return this.$store.state.accessGrantsModule.page.totalCount;
    }

    /**
     * bucketsAmount returns buckets amount for selected project.
     */
    public get bucketsAmount(): number {
        return this.$store.state.bucketUsageModule.page.totalCount;
    }

    /**
     * estimatedCharges returns estimated charges summary for selected project.
     */
    public get estimatedCharges(): number {
        return this.$store.state.paymentsModule.priceSummaryForSelectedProject;
    }
}
</script>

<style scoped lang="scss">
    .project-summary {
        margin-top: 30px;
        padding: 20px;
        width: calc(100% - 40px);
        background-color: #fff;
        border-radius: 6px;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-style: normal;
            font-size: 16px;
            line-height: 16px;
            color: #1b2533;
            margin-bottom: 25px;
        }

        &__items {
            display: flex;
            align-items: center;
        }
    }

    .right-indent {
        margin-right: 20px;
    }
</style>
