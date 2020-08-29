// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="usage-charges-item-container">
        <div class="usage-charges-item-container__summary" @click="toggleDetailedInfo">
            <div class="usage-charges-item-container__summary__name-container">
                <ChargesExpandIcon v-if="isDetailedInfoShown"/>
                <ChargesHideIcon v-else/>
                <span>{{ projectName }}</span>
            </div>
            <span class="usage-charges-item-container__summary__amount">
                Estimated Total {{ item.summary() | centsToDollars }}
            </span>
        </div>
        <div class="usage-charges-item-container__detailed-info-container" v-if="isDetailedInfoShown">
            <div class="usage-charges-item-container__detailed-info-container__info-header">
                <span class="resource-header">RESOURCE</span>
                <span class="period-header">PERIOD</span>
                <span class="usage-header">USAGE</span>
                <span class="cost-header">COST</span>
            </div>
            <div class="usage-charges-item-container__detailed-info-container__content-area">
                <div class="usage-charges-item-container__detailed-info-container__content-area__resource-container">
                    <p>Storage ($0.010 per Gigabyte-Month)</p>
                    <p>Egress ($0.045 per GB)</p>
                    <p>Objects ($0.0000022 per Object-Month)</p>
                </div>
                <div class="usage-charges-item-container__detailed-info-container__content-area__period-container">
                    <p>{{ period }}</p>
                    <p>{{ period }}</p>
                    <p>{{ period }}</p>
                </div>
                <div class="usage-charges-item-container__detailed-info-container__content-area__usage-container">
                    <p>{{ storageFormatted }} Gigabyte-month</p>
                    <p>{{ egressAmountAndDimension }}</p>
                    <p>{{ objectCountFormatted }} Object-month</p>
                </div>
                <div class="usage-charges-item-container__detailed-info-container__content-area__cost-container">
                    <p class="price">{{ item.storagePrice | centsToDollars }}</p>
                    <p class="price">{{ item.egressPrice | centsToDollars }}</p>
                    <p class="price">{{ item.objectPrice | centsToDollars }}</p>
                </div>
            </div>
            <div class="usage-charges-item-container__detailed-info-container__link-container">
                <span class="usage-charges-item-container__detailed-info-container__link-container__link" @click="onReportClick">
                    Advanced Report ->
                </span>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import ChargesExpandIcon from '@/../static/images/account/billing/chargesExpand.svg';
import ChargesHideIcon from '@/../static/images/account/billing/chargesHide.svg';

import { ProjectUsageAndCharges } from '@/types/payments';
import { Project } from '@/types/projects';
import { Size } from '@/utils/bytesSize';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { Time } from '@/utils/time';

@Component({
    components: {
        ChargesHideIcon,
        ChargesExpandIcon,
    },
})
export default class UsageAndChargesItem extends Vue {
    /**
     * item represents usage and charges of current project by period.
     */
    @Prop({default: () => new ProjectUsageAndCharges()})
    private readonly item: ProjectUsageAndCharges;

    /**
     * HOURS_IN_MONTH constant shows amount of hours in 30-day month.
     */
    private readonly HOURS_IN_MONTH: number = 720;

    /**
     * projectName returns project name.
     */
    public get projectName(): string {
        const projects: Project[] = this.$store.state.projectsModule.projects;
        const project: Project | undefined = projects.find(project => project.id === this.item.projectId);

        return project ? project.name : '';
    }

    /**
     * Returns string of date range.
     */
    public get period(): string {
        const since: string = `${SHORT_MONTHS_NAMES[this.item.since.getUTCMonth()]} ${this.item.since.getUTCDate()}`;
        const before: string = `${SHORT_MONTHS_NAMES[this.item.before.getUTCMonth()]} ${this.item.before.getUTCDate()}`;

        return `${since} - ${before}`;
    }

    /**
     * Returns string of egress amount and dimension.
     */
    public get egressAmountAndDimension(): string {
        return `${this.egressFormatted.formattedBytes} ${this.egressFormatted.label}`;
    }

    /**
     * Returns formatted storage used in GB x month dimension.
     */
    public get storageFormatted(): string {
        const bytesInGB: number = 1000000000;

        return (this.item.storage / this.HOURS_IN_MONTH / bytesInGB).toFixed(2);
    }

    /**
     * Returns formatted object count in object x month dimension.
     */
    public get objectCountFormatted(): string {
        return (this.item.objectCount / this.HOURS_IN_MONTH).toFixed(2);
    }

    /**
     * isDetailedInfoShown indicates if area with detailed information about project charges is expanded.
     */
    public isDetailedInfoShown: boolean = false;

    /**
     * toggleDetailedInfo expands an area with detailed information about project charges.
     */
    public toggleDetailedInfo(): void {
        this.isDetailedInfoShown = !this.isDetailedInfoShown;
    }

    /**
     * Opens new tab with advanced report table.
     */
    public onReportClick(): void {
        const projectID = this.$store.getters.selectedProject.id;
        const startDate = this.$store.state.paymentsModule.startDate;
        const endDate = this.$store.state.paymentsModule.endDate;

        const url = new URL(location.origin);

        url.pathname = 'usage-report';
        url.searchParams.append('projectID', projectID);
        url.searchParams.append('since', Time.toUnixTimestamp(startDate).toString());
        url.searchParams.append('before', Time.toUnixTimestamp(endDate).toString());

        this.$segment.track(SegmentEvent.REPORT_DOWNLOADED, {
            start_date: startDate,
            end_date: endDate,
            project_id: projectID,
        });

        window.open(url.href, '_blank');
    }

    /**
     * Returns formatted egress depending on amount of bytes.
     */
    private get egressFormatted(): Size {
        return new Size(this.item.egress, 2);
    }
}
</script>

<style scoped lang="scss">
    p {
        margin: 0;
    }

    .usage-charges-item-container {
        font-size: 16px;
        line-height: 21px;
        padding: 30px 0;
        font-family: 'font_regular', sans-serif;

        &__summary {
            display: flex;
            justify-content: space-between;
            align-items: center;
            cursor: pointer;

            &__name-container {
                display: flex;
                align-items: center;

                &__expand-image,
                &__hide-image {
                    width: 14px;
                    height: 14px;
                    margin-right: 12px;
                }

                &__expand-image {
                    height: 8px;
                }
            }

            &__amount {
                font-size: 16px;
                line-height: 21px;
                text-align: right;
                color: #354049;
            }
        }

        &__detailed-info-container {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            justify-content: space-between;
            padding: 16px 0 0 26px;

            &__info-header {
                display: flex;
                align-items: center;
                justify-content: space-between;
                font-size: 14px;
                line-height: 19px;
                color: #adadad;
                height: 25px;
                width: 100%;
            }

            &__content-area {
                width: 100%;
                padding: 10px 0 0 0;
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__resource-container,
                &__period-container,
                &__cost-container,
                &__usage-container {
                    width: 20%;
                    font-size: 14px;
                    line-height: 19px;
                    color: #354049;

                    :nth-child(1),
                    :nth-child(2) {
                        margin-bottom: 3px;
                    }
                }

                &__resource-container {
                    width: 40%;
                }
            }

            &__link-container {
                width: 100%;
                display: flex;
                justify-content: flex-end;
                align-items: center;
                margin-top: 25px;

                &__link {
                    font-size: 13px;
                    line-height: 19px;
                    color: #2683ff;
                    cursor: pointer;
                }
            }
        }
    }

    .resource-header {
        width: 40%;
    }

    .cost-header,
    .period-header,
    .usage-header {
        width: 20%;
    }

    .cost-header,
    .price {
        text-align: right;
    }
</style>
