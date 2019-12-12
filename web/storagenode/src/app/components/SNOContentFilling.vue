// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info-area">
        <SatelliteSelection/>
        <div v-if="isDisqualifiedInfoShown" class="info-area__disqualified-info">
            <LargeDisqualificationIcon
                class="info-area__disqualified-info__image"
                alt="Disqualified image"
            />
            <p class="info-area__disqualified-info__info">Your node has been paused on <b>{{getDisqualificationDate}}</b>. If you have any questions regarding this please contact our <a href="https://support.storj.io/hc/en-us/requests/new">support</a>.</p>
        </div>
        <div v-else-if="doDisqualifiedSatellitesExist" class="info-area__disqualified-info">
            <LargeDisqualificationIcon
                class="info-area__disqualified-info__image"
                alt="Disqualified image"
            />
            <p class="info-area__disqualified-info__info">Your node has been paused on<span v-for="disqualified in disqualifiedSatellites"><b> {{disqualified.id}}</b></span>. If you have any questions regarding this please contact our <a href="https://support.storj.io/hc/en-us/requests/new">support</a>.</p>
        </div>
        <p class="info-area__title">Utilization & Remaining</p>
        <div class="info-area__chart-area">
            <div class="chart-container">
                <div class="chart-container__title-area">
                    <p class="chart-container__title-area__title">Bandwidth Used This Month</p>
                    <div class="chart-container__title-area__chart-choice-item" :class="{'egress-chart-shown' : isEgressChartShown}" @click.stop="toggleEgressChartShowing">Egress</div>
                    <div class="chart-container__title-area__chart-choice-item" :class="{'ingress-chart-shown' : isIngressChartShown}" @click.stop="toggleIngressChartShowing">Ingress</div>
                </div>
                <p class="chart-container__amount" v-if="isBandwidthChartShown"><b>{{bandwidthSummary}}</b></p>
                <p class="chart-container__amount" v-if="isEgressChartShown"><b>{{egressSummary}}</b></p>
                <p class="chart-container__amount" v-if="isIngressChartShown"><b>{{ingressSummary}}</b></p>
                <div class="chart-container__chart">
                    <BandwidthChart v-if="isBandwidthChartShown"/>
                    <EgressChart v-if="isEgressChartShown"/>
                    <IngressChart v-if="isIngressChartShown"/>
                </div>
            </div>
            <div class="chart-container">
                <div class="chart-container__title-area disk-space-title">
                    <p class="chart-container__title-area__title">Disk Space Used This Month</p>
                </div>
                <p class="chart-container__amount disk-space-amount"><b>{{storageSummary}}*h</b></p>
                <div class="chart-container__chart">
                    <DiskSpaceChart/>
                </div>
            </div>
        </div>
        <div>
            <div class="info-area__remaining-space-area">
                <BarInfo
                    label="Bandwidth Remaining"
                    :amount="bandwidth.remaining"
                    info-text="of bandwidth left"
                    :current-bar-amount="bandwidth.used"
                    :max-bar-amount="bandwidth.available"
                />
                <BarInfo
                    label="Disk Space Remaining"
                    :amount="diskSpace.remaining"
                    info-text="of disk space left"
                    :current-bar-amount="diskSpace.used"
                    :max-bar-amount="diskSpace.available"
                />
            </div>
        </div>
        <div class="info-area__blurred-checks" v-if="!selectedSatellite.id">
            <p class="info-area__blurred-checks__title">Select A Specific Satellite To View Audit And Uptime Percentages</p>
        </div>
        <div v-if="selectedSatellite.id">
            <p class="info-area__title">Uptime & Audit Checks by Satellite</p>
            <div class="info-area__checks-area">
                <ChecksArea
                    label="Uptime Checks"
                    :amount="checks.uptime"
                    info-text="Uptime checks occur to make sure  your node is still online. This is the percentage of uptime checks you’ve passed."
                />
                <ChecksArea
                    label="Audit Checks"
                    :amount="checks.audit"
                    info-text="Percentage of successful pings/communication between the node & satellite."
                />
            </div>
        </div>
        <p class="info-area__title">Payout</p>
        <PayoutArea
            label="STORJ Wallet Address"
            :wallet-address="wallet"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BandwidthChart from '@/app/components/BandwidthChart.vue';
import BarInfo from '@/app/components/BarInfo.vue';
import ChecksArea from '@/app/components/ChecksArea.vue';
import DiskSpaceChart from '@/app/components/DiskSpaceChart.vue';
import EgressChart from '@/app/components/EgressChart.vue';
import IngressChart from '@/app/components/IngressChart.vue';
import PayoutArea from '@/app/components/PayoutArea.vue';
import SatelliteSelection from '@/app/components/SatelliteSelection.vue';

import LargeDisqualificationIcon from '@/../static/images/largeDisqualify.svg';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { formatBytes } from '@/app/utils/converter';
import { BandwidthInfo, DiskSpaceInfo, SatelliteInfo } from '@/storagenode/dashboard';

/**
 * Checks class holds info for Checks entity.
 */
class Checks {
    public uptime: number;
    public audit: number;

    public constructor(uptime: number, audit: number) {
        this.uptime = uptime;
        this.audit = audit;
    }
}

@Component ({
    components: {
        EgressChart,
        IngressChart,
        SatelliteSelection,
        BandwidthChart,
        DiskSpaceChart,
        BarInfo,
        ChecksArea,
        PayoutArea,
        LargeDisqualificationIcon,
    },
})
export default class SNOContentFilling extends Vue {
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
     * wallet - wallet address as string from store.
     * @return string - wallet address
     */
    public get wallet(): string {
        return this.$store.state.node.info.wallet;
    }

    /**
     * bandwidthSummary - amount of monthly bandwidth used from store.
     * @return string - formatted amount of monthly bandwidth used
     */
    public get bandwidthSummary(): string {
        return formatBytes(this.$store.state.node.bandwidthSummary);
    }

    /**
     * egressSummary - amount of monthly egress used from store.
     * @return string - formatted amount of monthly egress used
     */
    public get egressSummary(): string {
        return formatBytes(this.$store.state.node.egressSummary);
    }

    /**
     * ingressSummary - amount of monthly ingress used from store.
     * @return string - formatted amount of monthly ingress used
     */
    public get ingressSummary(): string {
        return formatBytes(this.$store.state.node.ingressSummary);
    }

    /**
     * storageSummary - amount of monthly disk space used from store.
     * @return string - formatted amount of monthly disk space used
     */
    public get storageSummary(): string {
        return formatBytes(this.$store.state.node.storageSummary);
    }

    /**
     * bandwidth - remaining amount of bandwidth from store.
     * @return BandwidthInfo - remaining amount of bandwidth
     */
    public get bandwidth(): BandwidthInfo {
        return this.$store.state.node.utilization.bandwidth;
    }

    /**
     * diskSpace - remaining amount of diskSpace from store.
     * @return DiskSpaceInfo - remaining amount of diskSpace
     */
    public get diskSpace(): DiskSpaceInfo {
        return this.$store.state.node.utilization.diskSpace;
    }

    /**
     * checks - uptime and audit checks statuses from store.
     * @return Checks - uptime and audit checks statuses
     */
    public get checks(): Checks {
        return this.$store.state.node.checks;
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
}
</script>

<style scoped lang="scss">
    p {
        margin-block-start: 0;
        margin-block-end: 0;
    }

    .info-area {
        width: 100%;
        padding: 0 0 30px 0;

        &__disqualified-info {
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
                min-width: 35px;
                margin-right: 17px;
            }

            &__info {
                font-size: 14px;
                line-height: 21px;
            }
        }

        &__title {
            font-size: 18px;
            line-height: 57px;
            color: #535f77;
            user-select: none;
        }

        &__blurred-checks {
            display: flex;
            align-items: center;
            justify-content: center;
            width: 100%;
            height: 224px;
            background-image: url('../../../static/images/BlurredChecks.png');
            background-size: contain;
            margin: 35px 0;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 49px;
                color: #4a4a4a;
                user-select: none;
            }
        }

        &__chart-area,
        &__remaining-space-area,
        &__checks-area {
            display: flex;
            justify-content: space-between;
        }
    }

    .chart-container {
        width: 339px;
        height: 336px;
        background-color: #fff;
        border: 1px solid #e9eff4;
        border-radius: 11px;
        padding: 32px 30px;
        margin-bottom: 13px;
        position: relative;

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__title {
                font-size: 14px;
                color: #586c86;
                user-select: none;
            }

            &__chart-choice-item {
                padding: 5px 12px;
                background-color: #f1f6ff;
                border-radius: 47px;
                font-size: 12px;
                color: #9daed2;
                max-height: 25px;
                cursor: pointer;
                user-select: none;
            }
        }

        &__amount {
            font-size: 32px;
            line-height: 57px;
            color: #535f77;
        }

        &__chart {
            position: absolute;
            bottom: 0;
            left: 0;
        }
    }

    .egress-chart-shown {
        background-color: #d3f2cc;
        color: #2e5f46;
    }

    .ingress-chart-shown {
        background-color: #ffeac2;
        color: #c48c4b;
    }

    .disk-space-title,
    .disk-space-amount {
        margin-top: 5px;
    }
</style>
