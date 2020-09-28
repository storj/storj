// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <section class="project-summary">
        <h1 class="project-summary__title">Details</h1>
        <div class="project-summary__items">
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
                title="API Keys"
                :value="apiKeysAmount"
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
import { Component, Vue } from 'vue-property-decorator';

import SummaryItem from '@/components/project/summary/SummaryItem.vue';

import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';

@Component({
    components: {
        SummaryItem,
    },
})
export default class ProjectSummary extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Fetches buckets and project usage and charges for current rollup.
     */
    public async mounted(): Promise<void> {
        if (!this.$store.getters.selectedProject.id) return;

        const FIRST_PAGE = 1;

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, FIRST_PAGE);
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * teamSize returns project members amount for selected project.
     */
    public get teamSize(): number {
        return this.$store.state.projectMembersModule.page.totalCount;
    }

    /**
     * apiKeysAmount returns API keys amount for selected project.
     */
    public get apiKeysAmount(): number {
        return this.$store.state.apiKeysModule.page.totalCount;
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
        padding: 25px;
        width: calc(100% - 50px);
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
