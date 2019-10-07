// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info-area">
        <SatelliteSelection/>
        <div v-if="isDisqualifiedInfoShown" class="info-area__disqualified-info">
            <svg class="info-area__disqualified-info__image" width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg" alt="Disqualified image">
                <path d="M37.0279 30.9608C36.5357 30.0515 36.0404 29.1405 35.5467 28.2265C34.6279 26.5312 33.7108 24.8358 32.7936 23.1405C31.692 21.1092 30.5936 19.0749 29.4936 17.0437C28.4311 15.0828 27.3717 13.1249 26.3092 11.1657C25.528 9.72504 24.7498 8.28289 23.9686 6.84088C23.7576 6.45026 23.5467 6.05964 23.3358 5.67212C23.117 5.26588 22.8858 4.87525 22.5545 4.54401C21.3983 3.37993 19.4795 3.15648 18.0889 4.0362C17.492 4.41433 17.0608 4.95028 16.7296 5.56432C16.2218 6.50184 15.7139 7.43933 15.2061 8.37996C14.2811 10.0909 13.3546 11.8018 12.4296 13.5128C11.3155 15.555 10.2108 17.602 9.10144 19.6488C8.05144 21.5894 6.99988 23.5269 5.94832 25.4692C5.17956 26.891 4.40924 28.3098 3.63896 29.7316C3.43584 30.1066 3.23272 30.4816 3.0296 30.8566C2.74523 31.3847 2.5218 31.919 2.45148 32.5284C2.25305 34.2503 3.45928 35.9472 5.12648 36.3691C5.56712 36.4816 6.00148 36.4863 6.44681 36.4863H33.9468H33.9906C34.8968 36.4675 35.7562 36.1269 36.4202 35.5097C37.0609 34.916 37.4359 34.1035 37.5421 33.2441C37.6437 32.4347 37.4093 31.6691 37.028 30.9613L37.0279 30.9608ZM18.4371 13.9528C18.4371 13.0778 19.1528 12.4294 19.9996 12.3904C20.8434 12.3513 21.5621 13.1372 21.5621 13.9528V24.956C21.5621 25.831 20.8464 26.4795 19.9996 26.5185C19.1558 26.5576 18.4371 25.7716 18.4371 24.956V13.9528ZM19.9996 31.8404C19.1215 31.8404 18.409 31.1295 18.409 30.2498C18.409 29.3717 19.1199 28.6592 19.9996 28.6592C20.8777 28.6592 21.5902 29.3701 21.5902 30.2498C21.5902 31.1279 20.8778 31.8404 19.9996 31.8404Z" fill="#F4D638"/>
            </svg>
            <p class="info-area__disqualified-info__info">Your node has been paused on <b>{{getDisqualificationDate}}</b>. If you have any questions regarding this please contact our <a href="https://support.storj.io/hc/en-us/requests/new">support</a>.</p>
        </div>
        <div v-else-if="doDisqualifiedSatellitesExist" class="info-area__disqualified-info">
            <svg class="info-area__disqualified-info__image" width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg" alt="Disqualified image">
                <path d="M37.0279 30.9608C36.5357 30.0515 36.0404 29.1405 35.5467 28.2265C34.6279 26.5312 33.7108 24.8358 32.7936 23.1405C31.692 21.1092 30.5936 19.0749 29.4936 17.0437C28.4311 15.0828 27.3717 13.1249 26.3092 11.1657C25.528 9.72504 24.7498 8.28289 23.9686 6.84088C23.7576 6.45026 23.5467 6.05964 23.3358 5.67212C23.117 5.26588 22.8858 4.87525 22.5545 4.54401C21.3983 3.37993 19.4795 3.15648 18.0889 4.0362C17.492 4.41433 17.0608 4.95028 16.7296 5.56432C16.2218 6.50184 15.7139 7.43933 15.2061 8.37996C14.2811 10.0909 13.3546 11.8018 12.4296 13.5128C11.3155 15.555 10.2108 17.602 9.10144 19.6488C8.05144 21.5894 6.99988 23.5269 5.94832 25.4692C5.17956 26.891 4.40924 28.3098 3.63896 29.7316C3.43584 30.1066 3.23272 30.4816 3.0296 30.8566C2.74523 31.3847 2.5218 31.919 2.45148 32.5284C2.25305 34.2503 3.45928 35.9472 5.12648 36.3691C5.56712 36.4816 6.00148 36.4863 6.44681 36.4863H33.9468H33.9906C34.8968 36.4675 35.7562 36.1269 36.4202 35.5097C37.0609 34.916 37.4359 34.1035 37.5421 33.2441C37.6437 32.4347 37.4093 31.6691 37.028 30.9613L37.0279 30.9608ZM18.4371 13.9528C18.4371 13.0778 19.1528 12.4294 19.9996 12.3904C20.8434 12.3513 21.5621 13.1372 21.5621 13.9528V24.956C21.5621 25.831 20.8464 26.4795 19.9996 26.5185C19.1558 26.5576 18.4371 25.7716 18.4371 24.956V13.9528ZM19.9996 31.8404C19.1215 31.8404 18.409 31.1295 18.409 30.2498C18.409 29.3717 19.1199 28.6592 19.9996 28.6592C20.8777 28.6592 21.5902 29.3701 21.5902 30.2498C21.5902 31.1279 20.8778 31.8404 19.9996 31.8404Z" fill="#F4D638"/>
            </svg>
            <p class="info-area__disqualified-info__info">Your node has been paused on<span v-for="disqualified in disqualifiedSatellites"><b> {{disqualified.id}}</b></span>. If you have any questions regarding this please contact our <a href="https://support.storj.io/hc/en-us/requests/new">support</a>.</p>
        </div>
        <p class="info-area__title">Utilization & Remaining</p>
        <div class="info-area__chart-area">
            <div class="chart-container">
                <p class="chart-container__title">Bandwidth Used This Month</p>
                <p class="chart-container__amount"><b>{{bandwidthSummary}}</b></p>
                <div class="chart-container__chart">
                    <BandwidthChart/>
                </div>
            </div>
            <div class="chart-container">
                <p class="chart-container__title">Disk Space Used This Month</p>
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
import PayoutArea from '@/app/components/PayoutArea.vue';
import SatelliteSelection from '@/app/components/SatelliteSelection.vue';
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
        SatelliteSelection,
        BandwidthChart,
        DiskSpaceChart,
        BarInfo,
        ChecksArea,
        PayoutArea,
    },
})
export default class SNOContentFilling extends Vue {
    public get wallet(): string {
        return this.$store.state.node.info.wallet;
    }

    public get bandwidthSummary(): string {
        return formatBytes(this.$store.state.node.bandwidthSummary);
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

<style lang="scss">
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
            background-color: #FCF8E3;
            border-radius: 12px;
            width: calc(100% - 52px);
            margin-top: 17px;

            &__image {
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
            color: #535F77;
        }

        &__chart-area,
        &__remaining-space-area,
        &__checks-area {
            display: flex;
            justify-content: space-between;
        }
    }

    .chart-container {
        width: 325px;
        height: 257px;
        background-color: #FFFFFF;
        border: 1px solid #E9EFF4;
        border-radius: 11px;
        padding: 34px 36px 39px 39px;
        margin-bottom: 32px;
        position: relative;

        &__title {
            font-size: 14px;
            color: #586C86;
        }

        &__amount {
            font-size: 32px;
            line-height: 57px;
            color: #535F77;
        }

        &__chart {
            position: absolute;
            bottom: 0;
            left: 0;
        }
    }
</style>
