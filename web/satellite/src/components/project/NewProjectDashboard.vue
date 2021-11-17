// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-dashboard">
        <h1 class="project-dashboard__title">Dashboard</h1>
        <VLoader v-if="isDataFetching" class="project-dashboard__loader" width="100px" height="100px" />
        <p v-else class="project-dashboard__subtitle">
            Your
            <span class="project-dashboard__subtitle__value">{{ limits.objectCount }} objects</span>
            are stored in
            <span class="project-dashboard__subtitle__value">{{ limits.segmentCount }} segments</span>
            around the world
        </p>
        <div class="project-dashboard__info">
            <InfoContainer
                title="Billing"
                :subtitle="status"
                :value="estimatedCharges | centsToDollars"
                :is-data-fetching="isDataFetching"
            >
                <template #side-value>
                    <p class="project-dashboard__info__label">Will be charged during next billing period</p>
                </template>
            </InfoContainer>
            <InfoContainer
                class="project-dashboard__info__middle"
                title="Objects"
                :subtitle="`Updated ${now}`"
                :value="limits.objectCount"
                :is-data-fetching="isDataFetching"
            >
                <template #side-value>
                    <p class="project-dashboard__info__label">Total of {{ usedFormatted }}</p>
                </template>
            </InfoContainer>
            <InfoContainer
                title="Segments"
                :subtitle="`Updated ${now}`"
                :value="limits.segmentCount"
                :is-data-fetching="isDataFetching"
            >
                <template #side-value>
                    <a
                        class="project-dashboard__info__link"
                        href="https://docs.storj.io/dcs/billing-payment-and-accounts-1/pricing/billing-and-payment"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        Learn more ->
                    </a>
                </template>
            </InfoContainer>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PROJECTS_ACTIONS } from "@/store/modules/projects";
import { PAYMENTS_ACTIONS } from "@/store/modules/payments";
import { RouteConfig } from "@/router";
import { ProjectLimits } from "@/types/projects";
import { Dimensions, Size } from "@/utils/bytesSize";

import VLoader from "@/components/common/VLoader.vue";
import InfoContainer from "@/components/project/InfoContainer.vue";

// @vue/component
@Component({
    components: {
        VLoader,
        InfoContainer,
    }
})
export default class NewProjectDashboard extends Vue {
    public now = new Date().toLocaleDateString('en-US');
    public isDataFetching = true;

    /**
     * Lifecycle hook after initial render.
     * Fetches project limits.
     */
    public async mounted(): Promise<void> {
        if (!this.$store.getters.selectedProject.id) {
            await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);

            return;
        }

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);

            this.isDataFetching = false;
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Returns current limits from store.
     */
    public get limits(): ProjectLimits {
        return this.$store.state.projectsModule.currentLimits;
    }

    /**
     * Returns status string based on account status.
     */
    public get status(): string {
        return this.$store.getters.user.paidTier ? 'Pro Account' : 'Free Account';
    }

    /**
     * estimatedCharges returns estimated charges summary for selected project.
     */
    public get estimatedCharges(): number {
        return this.$store.state.paymentsModule.priceSummaryForSelectedProject;
    }

    /**
     * Returns formatted used amount.
     */
    public get usedFormatted(): string {
        return this.formattedValue(new Size(this.limits.storageUsed, 2));
    }

    /**
     * Formats value to needed form and returns it.
     */
    private formattedValue(value: Size): string {
        switch (value.label) {
        case Dimensions.Bytes:
        case Dimensions.KB:
            return '0';
        default:
            return `${value.formattedBytes.replace(/\\.0+$/, '')}${value.label}`;
        }
    }
}
</script>

<style scoped lang="scss">
    .project-dashboard {
        padding: 56px 40px;
        background-image: url('../../../static/images/project/background.png');
        background-position: top right;
        background-size: 70%;
        background-repeat: no-repeat;

        &__loader {
            display: inline-block;
        }

        &__title {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            line-height: 24px;
            color: #000;
            margin-bottom: 64px;
        }

        &__subtitle {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 36px;
            letter-spacing: -0.02em;
            color: #000;
            max-width: 365px;

            &__value {
                text-decoration: underline;
                text-underline-position: under;
                text-decoration-color: #00e366;
            }
        }

        &__info {
            display: flex;
            align-items: center;
            margin-top: 100px;

            &__middle {
                margin: 0 16px;
            }

            &__label,
            &__link {
                font-weight: 500;
                font-size: 14px;
                line-height: 20px;
                color: #000;
            }

            &__link {
                text-decoration: underline !important;
                text-underline-position: under;

                &:visited {
                    color: #000;
                }
            }
        }
    }
</style>
