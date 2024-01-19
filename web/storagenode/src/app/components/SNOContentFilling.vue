// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info-area">
        <SatelliteSelection />
        <div class="info-area__announcement">
            <LargeInfoIcon
                class="info-area__announcement__image"
                alt="Announcement image"
            />
            <p class="info-area__announcement__info">
                Please read the information on the
                <a
                    class="info-area__announcement__info__link"
                    href="https://www.storj.io/s/storage-node-pricing-update-2023"
                    rel="noopener noreferrer"
                    target="_blank"
                >forum</a> regarding the Storage Node payout changes.
            </p>
        </div>
        <div v-if="isDisqualifiedInfoShown" class="info-area__disqualified-info">
            <LargeDisqualificationIcon
                class="info-area__disqualified-info__image"
                alt="Disqualified image"
            />
            <p class="info-area__disqualified-info__info">
                Your node has been disqualified on <b>{{ getDisqualificationDate }}</b>. If you have any questions regarding this please check our Node Operators
                <a
                    class="info-area__disqualified-info__info__link"
                    href="https://forum.storj.io/c/sno-category"
                    rel="noopener noreferrer"
                    target="_blank"
                >thread</a> on Storj forum.
            </p>
        </div>
        <div v-else-if="doDisqualifiedSatellitesExist" class="info-area__disqualified-info">
            <LargeDisqualificationIcon
                class="info-area__disqualified-info__image"
                alt="Disqualified image"
            />
            <p class="info-area__disqualified-info__info">
                Your node has been disqualified on<span v-for="disqualified in disqualifiedSatellites" :key="disqualified.id"><b> {{ disqualified.id }}</b></span>. If you have any questions regarding this please check our Node Operators
                <a
                    class="info-area__disqualified-info__info__link"
                    href="https://forum.storj.io/c/sno-category"
                    rel="noopener noreferrer"
                    target="_blank"
                >thread</a> on Storj forum.
            </p>
        </div>
        <div v-if="isSuspendedInfoShown" class="info-area__suspended-info">
            <LargeSuspensionIcon
                class="info-area__suspended-info__image"
                alt="Suspended image"
            />
            <p class="info-area__suspended-info__info">
                Your node has been suspended on <b>{{ getSuspensionDate }}</b>. If you have any questions regarding this please check our Node Operators
                <a
                    class="info-area__disqualified-info__info__link"
                    href="https://forum.storj.io/c/sno-category"
                    rel="noopener noreferrer"
                    target="_blank"
                >thread</a> on Storj forum.
            </p>
        </div>
        <div v-else-if="doSuspendedSatellitesExist" class="info-area__suspended-info">
            <LargeSuspensionIcon
                class="info-area__suspended-info__image"
                alt="Suspended image"
            />
            <p class="info-area__suspended-info__info">
                Your node has been suspended on<span v-for="suspended in suspendedSatellites" :key="suspended.id"><b> {{ suspended.id }}</b></span>. If you have any questions regarding this please check our Node Operators
                <a
                    class="info-area__disqualified-info__info__link"
                    href="https://forum.storj.io/c/sno-category"
                    rel="noopener noreferrer"
                    target="_blank"
                >thread</a> on Storj forum.
            </p>
        </div>
        <p class="info-area__title">Bandwidth Utilization </p>
        <section>
            <div class="chart-container bandwidth-chart">
                <div class="chart-container__title-area">
                    <p class="chart-container__title-area__title">Bandwidth Used This Month</p>
                    <div class="chart-container__title-area__buttons-area">
                        <button
                            name="Show Egress Chart"
                            class="chart-container__title-area__chart-choice-item"
                            type="button"
                            :class="{ 'egress-chart-shown': isEgressChartShown }"
                            @click.stop="toggleEgressChartShowing"
                        >
                            Egress
                        </button>
                        <button
                            name="Show Ingress Chart"
                            class="chart-container__title-area__chart-choice-item"
                            type="button"
                            :class="{ 'ingress-chart-shown': isIngressChartShown }"
                            @click.stop="toggleIngressChartShowing"
                        >
                            Ingress
                        </button>
                    </div>
                </div>
                <p v-if="isBandwidthChartShown" class="chart-container__amount"><b>{{ bandwidthSummary }}</b></p>
                <p v-if="isEgressChartShown" class="chart-container__amount"><b>{{ egressSummary }}</b></p>
                <p v-if="isIngressChartShown" class="chart-container__amount"><b>{{ ingressSummary }}</b></p>
                <div ref="chart" class="chart-container__chart" onresize="recalculateChartDimensions()">
                    <BandwidthChart v-if="isBandwidthChartShown" :height="chartHeight" :width="chartWidth" :is-dark-mode="isDarkMode" />
                    <EgressChart v-if="isEgressChartShown" :height="chartHeight" :width="chartWidth" :is-dark-mode="isDarkMode" />
                    <IngressChart v-if="isIngressChartShown" :height="chartHeight" :width="chartWidth" :is-dark-mode="isDarkMode" />
                </div>
            </div>
        </section>
        <p class="info-area__title">Disk Utilization & Remaining</p>
        <section class="info-area__chart-area">
            <section class="chart-container">
                <div class="chart-container__title-area disk-space-title">
                    <p class="chart-container__title-area__title">Average Disk Space Used This Month</p>
                </div>
                <p class="chart-container__amount disk-space-amount"><b>{{ averageUsageBytes }}</b></p>
                <div ref="diskSpaceChart" class="chart-container__chart" onresize="recalculateChartDimensions()">
                    <DiskSpaceChart :height="diskSpaceChartHeight" :width="diskSpaceChartWidth" :is-dark-mode="isDarkMode" />
                </div>
            </section>
            <section>
                <DiskStatChart />
            </section>
        </section>
        <div>
            <p class="info-area__title">Suspension & Audit</p>
            <div v-if="selectedSatellite.id" class="info-area__checks-area">
                <ChecksArea
                    label="Suspension Score"
                    :amount="audits.suspensionScore.label"
                    info-text="This score shows how close your node is to getting suspended on a satellite. A score of 96% or below will result in suspension. If your node stays suspended for more than one week you will be disqualified from this satellite, so please correct the errors that lead to suspension asap."
                />
                <ChecksArea
                    label="Audit Score"
                    :amount="audits.auditScore.label"
                    info-text="Percentage of successful pings/communication between the node & satellite."
                />
                <ChecksArea
                    label="Online Score"
                    :amount="audits.onlineScore.label"
                    info-text="Online checks occur to make sure your node is still online. This is the percentage of online checks you’ve passed."
                />
            </div>
            <AllSatellitesAuditsArea v-else />
        </div>
        <div class="info-area__payout-header">
            <p class="info-area__title">Payout</p>
            <router-link :to="PAYOUT_PATH" class="info-area__payout-header__link">
                <p class="info-area__payout-header__link__text">Payout Information</p>
                <BlueArrowRight />
            </router-link>
        </div>
        <WalletArea
            label="Wallet Address"
            :wallet-address="nodeInfo.wallet"
            :wallet-features="nodeInfo.walletFeatures"
        />
        <TotalPayoutArea class="info-area__total-area" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/app/router';
import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { Size } from '@/private/memory/size';
import { Dashboard, SatelliteInfo, SatelliteScores } from '@/storagenode/sno/sno';

import AllSatellitesAuditsArea from '@/app/components/AllSatellitesAuditsArea.vue';
import BandwidthChart from '@/app/components/BandwidthChart.vue';
import ChecksArea from '@/app/components/ChecksArea.vue';
import DiskSpaceChart from '@/app/components/DiskSpaceChart.vue';
import DiskStatChart from '@/app/components/DiskStatChart.vue';
import EgressChart from '@/app/components/EgressChart.vue';
import IngressChart from '@/app/components/IngressChart.vue';
import SatelliteSelection from '@/app/components/SatelliteSelection.vue';
import TotalPayoutArea from '@/app/components/TotalPayoutArea.vue';
import WalletArea from '@/app/components/WalletArea.vue';

import LargeSuspensionIcon from '@/../static/images/largeSuspend.svg';
import LargeDisqualificationIcon from '@/../static/images/largeDisqualify.svg';
import LargeInfoIcon from '@/../static/images/LargeInfo.svg';
import BlueArrowRight from '@/../static/images/BlueArrowRight.svg';

// @vue/component
@Component ({
    components: {
        AllSatellitesAuditsArea,
        DiskStatChart,
        TotalPayoutArea,
        EgressChart,
        IngressChart,
        SatelliteSelection,
        BandwidthChart,
        DiskSpaceChart,
        ChecksArea,
        WalletArea,
        LargeDisqualificationIcon,
        LargeSuspensionIcon,
        BlueArrowRight,
        LargeInfoIcon,
    },
})
export default class SNOContentFilling extends Vue {
    public readonly PAYOUT_PATH: string = RouteConfig.Payout.path;
    public chartWidth = 0;
    public chartHeight = 0;
    public diskSpaceChartWidth = 0;
    public diskSpaceChartHeight = 0;

    public $refs: {
        chart: HTMLElement;
        diskSpaceChart: HTMLElement;
    };

    public get isDarkMode(): boolean {
        return this.$store.state.appStateModule.isDarkMode;
    }

    /**
     * Used container size recalculation for charts resizing.
     */
    public recalculateChartDimensions(): void {
        this.chartWidth = this.$refs['chart'].clientWidth;
        this.chartHeight = this.$refs['chart'].clientHeight;
        this.diskSpaceChartWidth = this.$refs['diskSpaceChart'].clientWidth;
        this.diskSpaceChartHeight = this.$refs['diskSpaceChart'].clientHeight;
    }

    /**
     * Lifecycle hook after initial render.
     * Adds event on window resizing to recalculate size of charts.
     */
    public mounted(): void {
        window.addEventListener('resize', this.recalculateChartDimensions);
        this.recalculateChartDimensions();
    }

    /**
     * Lifecycle hook before component destruction.
     * Removes event on window resizing.
     */
    public beforeDestroy(): void {
        window.removeEventListener('resize', this.recalculateChartDimensions);
    }

    /**
     * isBandwidthChartShown showing status of bandwidth chart from store.
     * @return boolean - bandwidth chart displaying status
     */
    public get isBandwidthChartShown(): boolean {
        return this.$store.state.appStateModule.isBandwidthChartShown;
    }

    /**
     * isIngressChartShown showing status of ingress chart from store.
     * @return boolean - ingress chart displaying status
     */
    public get isIngressChartShown(): boolean {
        return this.$store.state.appStateModule.isIngressChartShown;
    }

    /**
     * isEgressChartShown showing status of egress chart from store.
     * @return boolean - egress chart displaying status
     */
    public get isEgressChartShown(): boolean {
        return this.$store.state.appStateModule.isEgressChartShown;
    }

    /**
     * toggleEgressChartShowing toggles displaying of egress chart.
     */
    public toggleEgressChartShowing(): void {
        if (this.isBandwidthChartShown || this.isIngressChartShown) {
            this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_EGRESS_CHART);

            return;
        }

        this.$store.dispatch(APPSTATE_ACTIONS.CLOSE_ADDITIONAL_CHARTS);
    }

    /**
     * toggleIngressChartShowing toggles displaying of ingress chart.
     */
    public toggleIngressChartShowing(): void {
        if (this.isBandwidthChartShown || this.isEgressChartShown) {
            this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_INGRESS_CHART);

            return;
        }

        this.$store.dispatch(APPSTATE_ACTIONS.CLOSE_ADDITIONAL_CHARTS);
    }

    /**
     * nodeInfo - contains common sno dashboard information.
     * @return Dashboard
     */
    public get nodeInfo(): Dashboard {
        return this.$store.state.node.info;
    }

    /**
     * bandwidthSummary - amount of monthly bandwidth used from store.
     * @return string - formatted amount of monthly bandwidth used
     */
    public get bandwidthSummary(): string {
        return Size.toBase10String(this.$store.state.node.bandwidthSummary);
    }

    /**
     * egressSummary - amount of monthly egress used from store.
     * @return string - formatted amount of monthly egress used
     */
    public get egressSummary(): string {
        return Size.toBase10String(this.$store.state.node.egressSummary);
    }

    /**
     * ingressSummary - amount of monthly ingress used from store.
     * @return string - formatted amount of monthly ingress used
     */
    public get ingressSummary(): string {
        return Size.toBase10String(this.$store.state.node.ingressSummary);
    }

    /**
     * storageSummary - amount of monthly disk space used from store.
     * @return string - formatted amount of monthly disk space used
     */
    public get averageUsageBytes(): string {
        return Size.toBase10String(this.$store.state.node.averageUsageBytes);
    }

    /**
     * checks - audit checks status from store.
     * @return Checks - audit checks statuses
     */
    public get audits(): SatelliteScores {
        return this.$store.state.node.audits;
    }

    /**
     * selectedSatellite - current selected satellite from store.
     * @return SatelliteInfo - current selected satellite
     */
    public get selectedSatellite(): SatelliteInfo {
        return this.$store.state.node.selectedSatellite;
    }

    /**
     * disqualifiedSatellites - array of disqualified satellites from store.
     * @return SatelliteInfo[] - array of disqualified satellites
     */
    public get disqualifiedSatellites(): SatelliteInfo[] {
        return this.$store.state.node.disqualifiedSatellites;
    }

    /**
     * isDisqualifiedInfoShown checks if disqualification status is shown.
     * @return boolean - disqualification status
     */
    public get isDisqualifiedInfoShown(): boolean {
        return !!(this.selectedSatellite.id && this.selectedSatellite.disqualified);
    }

    /**
     * getDisqualificationDate gets a date of disqualification.
     * @return String - date of disqualification
     */
    public get getDisqualificationDate(): string {
        if (this.selectedSatellite.disqualified) {
            return this.selectedSatellite.disqualified.toUTCString();
        }

        return '';
    }

    /**
     * doDisqualifiedSatellitesExist checks if disqualified satellites exist.
     * @return boolean - disqualified satellites existing status
     */
    public get doDisqualifiedSatellitesExist(): boolean {
        return this.disqualifiedSatellites.length > 0;
    }

    /**
     * suspendedSatellites - array of suspended satellites from store.
     * @return SatelliteInfo[] - array of suspended satellites
     */
    public get suspendedSatellites(): SatelliteInfo[] {
        return this.$store.state.node.suspendedSatellites;
    }

    /**
     * isSuspendedInfoShown checks if suspension status is shown.
     * @return boolean - suspension status
     */
    public get isSuspendedInfoShown(): boolean {
        return !!(this.selectedSatellite.id && this.selectedSatellite.suspended);
    }

    /**
     * getSuspensionDate gets a date of suspension.
     * @return String - date of suspension
     */
    public get getSuspensionDate(): string {
        if (this.selectedSatellite.suspended) {
            return this.selectedSatellite.suspended.toUTCString();
        }

        return '';
    }

    /**
     * doSuspendedSatellitesExist checks if suspended satellites exist.
     * @return boolean - suspended satellites existing status
     */
    public get doSuspendedSatellitesExist(): boolean {
        return this.suspendedSatellites.length > 0;
    }
}
</script>

<style scoped lang="scss">
    p {
        margin-block-start: 0;
        margin-block-end: 0;
    }

    .info-area {
        width: 100%;
        padding: 0 0 30px;

        &__disqualified-info {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 20px 27px 20px 25px;
            background-color: var(--block-background-color);
            border-radius: 12px;
            width: calc(100% - 52px);
            margin-top: 17px;
            color: var(--regular-text-color);

            &__image {
                min-height: 35px;
                min-width: 38px;
                margin-right: 17px;
            }

            &__info {
                font-size: 14px;
                line-height: 21px;

                &__link {
                    color: var(--navigation-link-color);
                }
            }
        }

        &__suspended-info {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 20px 27px 20px 25px;
            background-color: #fcf8e3;
            border-radius: 12px;
            width: calc(100% - 52px);
            margin-top: 17px;

            &__image {
                min-height: 35px;
                min-width: 38px;
                margin-right: 17px;
            }

            &__info {
                font-size: 14px;
                line-height: 21px;

                &__link {
                    color: var(--navigation-link-color);
                }
            }
        }

        &__announcement {
            display: flex;
            align-items: center;
            padding: 20px 27px 20px 25px;
            background-color: var(--block-background-color);
            border-radius: 12px;
            width: calc(100% - 52px);
            margin-top: 17px;
            color: var(--regular-text-color);

            &__image {
                min-height: 35px;
                min-width: 38px;
                margin-right: 17px;
            }

            &__info {
                font-size: 14px;
                line-height: 21px;

                &__link {
                    color: var(--navigation-link-color);
                }
            }
        }

        &__title {
            font-size: 18px;
            line-height: 57px;
            color: var(--regular-text-color);
            user-select: none;
        }

        &__blurred-checks {
            display: flex;
            align-items: center;
            justify-content: center;
            width: 100%;
            height: 224px;
            background-image: var(--blurred-image-path);
            background-size: contain;
            margin: 35px 0;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 49px;
                color: var(--regular-text-color);
                user-select: none;
            }
        }

        &__payout-header {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;

            &__link {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: flex-end;
                text-decoration: none;

                &__text {
                    font-size: 16px;
                    line-height: 22px;
                    color: var(--navigation-link-color);
                    margin-right: 9px;
                }
            }
        }

        &__chart-area,
        &__checks-area {
            display: flex;
            flex-direction: row;
            justify-content: space-between;
            width: 100%;
        }

        &__bar-info {
            width: 339px;
        }

        &__estimation-area {
            margin-top: 11px;
        }

        &__total-area {
            margin-top: 20px;
        }
    }

    .chart-container {
        width: 339px;
        height: 336px;
        background-color: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        border-radius: 11px;
        padding: 32px 30px;
        margin-bottom: 13px;
        position: relative;

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__buttons-area {
                display: flex;
                flex-direction: row;
                align-items: flex-end;
            }

            &__title {
                font-size: 14px;
                color: var(--regular-text-color);
                user-select: none;
            }

            &__chart-choice-item {
                padding: 5px 12px;
                background-color: var(--chart-selection-button-background-color);
                border-radius: 47px;
                font-size: 12px;
                color: #9daed2;
                max-height: 25px;
                cursor: pointer;
                user-select: none;
                margin-left: 9px;
            }
        }

        &__amount {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 57px;
            color: var(--regular-text-color);
        }

        &__chart {
            position: absolute;
            left: 0;
            width: calc(100% - 10px);
            height: 240px;
        }
    }

    .egress-chart-shown {
        background-color: var(--egress-button-background-color);
        color: var(--egress-button-font-color);
    }

    .ingress-chart-shown {
        background-color: var(--ingress-button-background-color);
        color: var(--ingress-button-font-color);
    }

    .disk-space-title,
    .disk-space-amount {
        margin-top: 5px;
    }

    .bandwidth-chart {
        width: calc(100% - 60px);
    }

    @media screen and (max-width: 1000px) {

        .info-area {

            &__chart-area {
                flex-direction: column;
                justify-content: flex-start;
            }
        }

        .chart-container {
            width: calc(100% - 60px);
        }
    }

    @media screen and (max-width: 780px) {

        .info-area {

            &__checks-area {
                flex-direction: column;

                .checks-area-container {
                    width: calc(100% - 60px) !important;
                }
            }

            &__blurred-checks {

                &__title {
                    text-align: center;
                }
            }
        }
    }

    @media screen and (max-width: 400px) {

        .chart-container {
            width: calc(100% - 36px);
            padding: 24px 18px;
        }
    }
</style>
