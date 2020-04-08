// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="usage-charges-item-container">
        <div class="usage-charges-item-container__summary" @click.self="toggleDetailedInfo">
            <div class="usage-charges-item-container__summary__name-container" @click="toggleDetailedInfo">
                <svg class="usage-charges-item-container__summary__name-container__expand-image" v-if="!isDetailedInfoShown" width="8" height="14" viewBox="0 0 8 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M0.328889 13.6272C-0.10963 13.1302 -0.10963 12.3243 0.328889 11.8273L4.58792 7L0.328889 2.17268C-0.10963 1.67565 -0.10963 0.869804 0.328889 0.372774C0.767408 -0.124258 1.47839 -0.124258 1.91691 0.372774L7.76396 7L1.91691 13.6272C1.47839 14.1243 0.767409 14.1243 0.328889 13.6272Z" fill="#2683FF"/>
                </svg>
                <svg class="usage-charges-item-container__summary__name-container__expand-image" v-if="isDetailedInfoShown" width="14" height="8" viewBox="0 0 14 8" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M0.372773 0.338888C0.869804 -0.112963 1.67565 -0.112963 2.17268 0.338888L7 4.72741L11.8273 0.338888C12.3243 -0.112963 13.1302 -0.112963 13.6272 0.338888C14.1243 0.790739 14.1243 1.52333 13.6272 1.97519L7 8L0.372773 1.97519C-0.124258 1.52333 -0.124258 0.790739 0.372773 0.338888Z" fill="#2683FF"/>
                </svg>
                <span>{{ projectName }}</span>
            </div>
            <div class="usage-charges-item-container__summary__report-link" @click="onReportClick">Advanced Report -></div>
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
            <span class="usage-charges-item-container__detailed-info-container__summary">{{ item.summary() | centsToDollars }}</span>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { ProjectUsageAndCharges } from '@/types/payments';
import { Project } from '@/types/projects';
import { Size } from '@/utils/bytesSize';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { toUnixTimestamp } from '@/utils/time';

@Component
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
        const months: string[] = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
        const since: string = `${months[this.item.since.getUTCMonth()]} ${this.item.since.getUTCDate()}`;
        const before: string = `${months[this.item.before.getUTCMonth()]} ${this.item.before.getUTCDate()}`;

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
        url.searchParams.append('since', toUnixTimestamp(startDate).toString());
        url.searchParams.append('before', toUnixTimestamp(endDate).toString());

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
        padding: 20px 0;
        font-family: 'font_regular', sans-serif;

        &__summary {
            display: flex;
            justify-content: space-between;
            align-items: center;
            cursor: pointer;

            &__name-container {
                display: flex;
                align-items: center;

                &__expand-image {
                    width: 14px;
                    height: 14px;
                    margin-right: 12px;
                }
            }

            &__report-link {
                padding: 3px 5px;
                font-size: 13px;
                line-height: 19px;
                color: #2683ff;
                background-color: rgba(38, 131, 255, 0.16);
                border-radius: 6px;
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
                border-bottom: 1px solid #b9b9b9;
                height: 25px;
                width: 100%;
            }

            &__content-area {
                width: 100%;
                padding: 10px 0;
                border-bottom: 1px solid #b9b9b9;
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

            &__summary {
                width: 100%;
                font-size: 14px;
                line-height: 18px;
                text-align: right;
                margin-top: 13px;
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
