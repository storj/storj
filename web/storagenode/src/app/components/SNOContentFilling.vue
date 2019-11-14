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
                    <div class="chart-container__title-area__chart-choice-item">Ingress</div>
                </div>
                <p class="chart-container__amount" v-if="!isEgressChartShown"><b>{{bandwidthSummary}}</b></p>
                <p class="chart-container__amount" v-if="isEgressChartShown"><b>{{egressBandwidthSummary}}</b></p>
                <p class="chart-container__amount" v-if="false"><b>{{ingressBandwidthSummary}}</b></p>
                <div class="chart-container__chart">
                    <BandwidthChart v-if="!isEgressChartShown"/>
                    <EgressChart v-if="isEgressChartShown"/>
                </div>
            </div>
            <div class="chart-container">
                <div class="chart-container__title-area">
                    <p class="chart-container__title-area__title">Disk Space Used This Month</p>
                </div>
                <p class="chart-container__amount"><b>{{storageSummary}}*h</b></p>
                <div class="chart-container__chart">
                    <DiskSpaceChart/>
                </div>
            </div>
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
        <div>
            <p class="info-area__title">Remaining on the Node</p>
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
    public get isEgressChartShown(): boolean {
        return this.$store.state.appStateModule.isEgressChartShown;
    }

    public toggleEgressChartShowing(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_EGRESS_CHART);
    }

    public get wallet(): string {
        return this.$store.state.node.info.wallet;
    }

    public get bandwidthSummary(): string {
        return formatBytes(this.$store.state.node.bandwidthSummary);
    }

    public get egressBandwidthSummary(): string {
        return formatBytes(this.$store.state.node.egressBandwidthSummary);
    }

    public get ingressBandwidthSummary(): string {
        return formatBytes(this.$store.state.node.ingressBandwidthSummary);
    }

    public get storageSummary(): string {
        return formatBytes(this.$store.state.node.storageSummary);
    }

    public get bandwidth(): BandwidthInfo {
        return this.$store.state.node.utilization.bandwidth;
    }

    public get diskSpace(): DiskSpaceInfo {
        return this.$store.state.node.utilization.diskSpace;
    }

    public get checks(): Checks {
        return this.$store.state.node.checks;
    }

    public get selectedSatellite(): SatelliteInfo {
        return this.$store.state.node.selectedSatellite;
    }

    public get disqualifiedSatellites(): SatelliteInfo[] {
        return this.$store.state.node.disqualifiedSatellites;
    }

    public get isDisqualifiedInfoShown(): boolean {
        return !!(this.selectedSatellite.id && this.selectedSatellite.disqualified);
    }

    public get getDisqualificationDate(): string {
        if (this.selectedSatellite.disqualified) {
            return this.selectedSatellite.disqualified.toUTCString();
        }

        return '';
    }

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
</style>
