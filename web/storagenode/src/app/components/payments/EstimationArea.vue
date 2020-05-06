// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="estimation-container">
        <div class="estimation-container__header">
            <p class="estimation-container__header__title">Info & Estimation</p>
            <EstimationPeriodDropdown />
        </div>
        <div class="estimation-container__divider"></div>
        <div class="estimation-table-container" v-if="!isPayoutNoDataState">
            <div class="estimation-table-container__labels-area">
                <div class="column justify-start column-1">
                    <p class="estimation-table-container__labels-area__text">Name</p>
                </div>
                <div class="column justify-start column-2">
                    <p class="estimation-table-container__labels-area__text">Type</p>
                </div>
                <div class="column justify-start column-3">
                    <p class="estimation-table-container__labels-area__text">Price</p>
                </div>
                <div class="column justify-start column-4">
                    <p class="estimation-table-container__labels-area__text">Disk</p>
                </div>
                <div class="column justify-start column-5">
                    <p class="estimation-table-container__labels-area__text">Bandwidth</p>
                </div>
                <div class="column justify-end column-6">
                    <p class="estimation-table-container__labels-area__text">Payout</p>
                </div>
            </div>
            <div v-for="item in tableData" :key="item.name" class="estimation-table-container__info-area">
                <div class="column justify-start column-1">
                    <p class="estimation-table-container__info-area__text">{{ item.name }}</p>
                </div>
                <div class="column justify-start column-2">
                    <p class="estimation-table-container__info-area__text">{{ item.type }}</p>
                </div>
                <div class="column justify-start column-3">
                    <p class="estimation-table-container__info-area__text">{{ item.price }}</p>
                </div>
                <div class="column justify-start column-4">
                    <p class="estimation-table-container__info-area__text">{{ item.disk }}</p>
                </div>
                <div class="column justify-start column-5">
                    <p class="estimation-table-container__info-area__text">{{ item.bandwidth }}</p>
                </div>
                <div class="column justify-end column-6">
                    <p class="estimation-table-container__info-area__text">{{ item.payout | centsToDollars }}</p>
                </div>
            </div>
            <div class="estimation-table-container__info-area" v-if="isCurrentPeriod">
                <div class="column justify-start column-1">
                    <p class="estimation-table-container__info-area__text">Gross Total</p>
                </div>
                <div class="column justify-start column-2"></div>
                <div class="column justify-start column-3"></div>
                <div class="column justify-start column-4"></div>
                <div class="column justify-start column-5"></div>
                <div class="column justify-end column-6">
                    <p class="estimation-table-container__info-area__text">{{ grossTotal | centsToDollars }}</p>
                </div>
            </div>
            <div class="estimation-table-container__held-area" v-if="!isCurrentPeriod || isSatelliteSelected">
                <p class="estimation-table-container__held-area__text">Held back</p>
                <p class="estimation-table-container__held-area__text">-{{ held | centsToDollars }}</p>
            </div>
            <div class="estimation-table-container__total-area">
                <div class="column justify-start column-1">
                    <p class="estimation-table-container__total-area__text">TOTAL</p>
                </div>
                <div class="column justify-start column-2"></div>
                <div class="column justify-start column-3"></div>
                <div class="column justify-start column-4">
                    <p class="estimation-table-container__total-area__text">{{ totalDiskSpace + 'h' }}</p>
                </div>
                <div class="column justify-start column-5">
                    <p class="estimation-table-container__total-area__text">{{ totalBandwidth }}</p>
                </div>
                <div class="column justify-end column-6">
                    <p class="estimation-table-container__total-area__text">{{ totalPayout | centsToDollars }}</p>
                </div>
            </div>
        </div>
        <div class="no-data-container" v-else>
            <img class="no-data-container__image" src="@/../static/images/payments/NoData.png">
            <p class="no-data-container__title">No data to display</p>
            <p class="no-data-container__additional-text">Please note, historical data about payouts does not update immediately, it may take some time.</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import EstimationPeriodDropdown from '@/app/components/payments/EstimationPeriodDropdown.vue';

import {
    BANDWIDTH_DOWNLOAD_PRICE_PER_TB,
    BANDWIDTH_REPAIR_PRICE_PER_TB,
    DISK_SPACE_PRICE_PER_TB,
} from '@/app/store/modules/payout';
import { HeldInfo } from '@/app/types/payout';
import { formatBytes, TB } from '@/app/utils/converter';

/**
 * Describes table row data item.
 */
class EstimationTableRow {
    public constructor(
        public name: string = '',
        public type: string = '',
        public price: string = '0',
        public disk: string = '',
        public bandwidth: string = '',
        public payout: number = 0,
    ) {}
}

@Component ({
    components: {
        EstimationPeriodDropdown,
    },
})
export default class EstimationArea extends Vue {
    public now: Date = new Date();

    /**
     * Indicates if current month selected.
     */
    public get isCurrentPeriod(): boolean {
        const end = this.$store.state.payoutModule.periodRange.end;
        const isCurrentMonthSelected = end.year === this.now.getUTCFullYear() && end.month === this.now.getUTCMonth();

        return !this.$store.state.payoutModule.periodRange.start && isCurrentMonthSelected;
    }

    public get isSatelliteSelected(): boolean {
        return !!this.$store.state.node.selectedSatellite.id;
    }

    /**
     * Indicates if payout data is unavailable.
     */
    public get isPayoutNoDataState(): boolean {
        return this.$store.state.appStateModule.isNoPayoutData;
    }

    /**
     * Returns held info from store.
     */
    public get heldInfo(): HeldInfo {
        return this.$store.state.payoutModule.heldInfo;
    }

    /**
     * Returns calculated or stored held amount.
     */
    public get held(): number {
        if (this.isCurrentPeriod) {
            return this.currentMonthHeld();
        }

        return this.heldInfo.held;
    }

    /**
     * Returns calculated or stored total payout by selected period.
     */
    public get totalPayout(): number {
        if (this.isCurrentPeriod) {
            return this.isSatelliteSelected ? this.grossTotal - this.held : this.grossTotal;
        }

        return this.heldInfo.paid;
    }

    /**
     * Returns calculated gross payout by selected period.
     */
    public get grossTotal(): number {
        return (this.currentBandwidthDownload * BANDWIDTH_DOWNLOAD_PRICE_PER_TB
            + this.currentBandwidthAuditAndRepair * BANDWIDTH_REPAIR_PRICE_PER_TB
            + this.currentDiskSpace * DISK_SPACE_PRICE_PER_TB / 730) / TB;
    }

    /**
     * Returns calculated or stored total used disk space by selected period.
     */
    public get totalDiskSpace(): string {
        if (this.isCurrentPeriod) {
            return formatBytes(this.currentDiskSpace);
        }

        return formatBytes(this.heldInfo.usageAtRest);
    }

    /**
     * Returns calculated or stored total used bandwidth by selected period.
     */
    public get totalBandwidth(): string {
        if (this.isCurrentPeriod) {
            return formatBytes((this.currentBandwidthAuditAndRepair + this.currentBandwidthDownload));
        }

        const bandwidthSum = this.heldInfo.usageGet + this.heldInfo.usageGetRepair + this.heldInfo.usageGetAudit;

        return formatBytes(bandwidthSum);
    }

    /**
     * Returns summary of current month audit and repair bandwidth.
     */
    private get currentBandwidthAuditAndRepair(): number {
        if (!this.$store.state.node.egressChartData) return 0;

        return this.$store.state.node.egressChartData.map(data => data.egress.audit + data.egress.repair).reduce((previous, current) => previous + current, 0);
    }

    /**
     * Returns summary of current month download bandwidth.
     */
    private get currentBandwidthDownload(): number {
        if (!this.$store.state.node.egressChartData) return 0;

        return this.$store.state.node.egressChartData.map(data => data.egress.usage)
            .reduce((previous, current) => previous + current, 0);
    }

    /**
     * Returns summary of current month used disk space.
     */
    private get currentDiskSpace(): number {
        if (!this.$store.state.node.storageChartData) return 0;

        return this.$store.state.node.storageChartData.map(data => data.atRestTotal).reduce((previous, current) => previous + current, 0);
    }

    /**
     * Builds estimated payout table depends on selected period.
     */
    public get tableData(): EstimationTableRow[] {
        if (!this.isCurrentPeriod) {
            return [
                new EstimationTableRow('Download', 'Egress', `$${BANDWIDTH_DOWNLOAD_PRICE_PER_TB / 100} / TB`, '--', formatBytes(this.heldInfo.usageGet), this.heldInfo.compGet),
                new EstimationTableRow('Repair & Audit', 'Egress', `$${BANDWIDTH_REPAIR_PRICE_PER_TB / 100} / TB`, '--', formatBytes(this.heldInfo.usageGetRepair + this.heldInfo.usageGetAudit), this.heldInfo.compGetRepair + this.heldInfo.compGetAudit),
                new EstimationTableRow('Disk Average Month', 'Storage', `$${DISK_SPACE_PRICE_PER_TB / 100} / TBm`, formatBytes(this.heldInfo.usageAtRest) + 'h', '--', this.heldInfo.compAtRest),
            ];
        }

        const approxHourInMonth = 730;

        return [
            new EstimationTableRow(
                'Download',
                'Egress',
                `$${BANDWIDTH_DOWNLOAD_PRICE_PER_TB / 100} / TB`,
                '--',
                formatBytes(this.currentBandwidthDownload),
                this.currentBandwidthDownload * BANDWIDTH_DOWNLOAD_PRICE_PER_TB / TB,
            ),
            new EstimationTableRow(
                'Repair & Audit',
                'Egress',
                `$${BANDWIDTH_REPAIR_PRICE_PER_TB / 100} / TB`,
                '--',
                formatBytes(this.currentBandwidthAuditAndRepair),
                this.currentBandwidthAuditAndRepair * BANDWIDTH_REPAIR_PRICE_PER_TB / TB,
            ),
            new EstimationTableRow(
                'Disk Average Month',
                'Storage',
                `$${DISK_SPACE_PRICE_PER_TB / 100} / TBm`,
                this.totalDiskSpace + 'h',
                '--',
                this.currentDiskSpace * DISK_SPACE_PRICE_PER_TB / TB / approxHourInMonth,
            ),
        ];
    }

    /**
     * Returns current month held amount based on currend day of month.
     */
    private currentMonthHeld(): number {
        return this.grossTotal * this.$store.state.payoutModule.heldPercentage / 100;
    }
}
</script>

<style scoped lang="scss">
    .estimation-container {
        display: flex;
        flex-direction: column;
        padding: 28px 40px 28px 40px;
        background: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        box-sizing: border-box;
        border-radius: 12px;
        font-family: 'font_regular', sans-serif;

        &__header {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;

            &__title {
                font-weight: 500;
                font-size: 18px;
                color: var(--regular-text-color);
            }
        }

        &__total-held,
        &__payout-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 87px;

            &__left-area {
                display: flex;
                flex-direction: column;
                align-items: flex-start;
            }

            &__right-area {
                display: flex;
                flex-direction: column;
                align-items: flex-end;
            }
        }

        &__total-held {
            border-bottom: 1px solid #eaeaea;
        }

        &__divider {
            width: 100%;
            height: 1px;
            margin-top: 18px;
            background-color: #eaeaea;
        }
    }

    .title-text {
        font-family: 'font_bold', sans-serif;
        font-size: 16px;
        line-height: 20px;
        color: var(--regular-text-color);
    }

    .additional-text {
        font-size: 13px;
        line-height: 17px;
        color: #b5bdcb;
    }

    .estimation-table-container {
        font-family: 'font_regular', sans-serif;

        &__labels-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: center;
            margin-top: 17px;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 36px;
            background: var(--estimation-table-header-color);

            &__text {
                font-weight: 500;
                font-size: 14px;
                color: #909bad;
            }
        }

        &__info-area {
            padding: 0 16px;
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: center;
            height: 56px;
            border-bottom: 1px solid #a9b5c1;

            &__text {
                font-size: 14px;
                color: var(--regular-text-color);
            }
        }

        &__held-area {
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 56px;
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;

            &__text {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                color: var(--regular-text-color);
            }
        }

        &__total-area {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 56px;
            background-color: var(--estimation-table-total-container-color);

            &__text {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                color: var(--regular-text-color);
            }
        }
    }

    .column {
        display: flex;
        flex-direction: row;
        align-items: center;
    }

    .justify-start {
        justify-content: flex-start;
    }

    .justify-end {
        justify-content: flex-end;
    }

    .column-1 {
        width: 26.9%;
    }

    .column-2 {
        width: 14.3%;
    }

    .column-3 {
        width: 13%;
    }

    .column-4 {
        width: 18.2%;
    }

    .column-5 {
        width: 18.9%;
    }

    .column-6 {
        width: 8.7%;
    }

    .no-data-container {
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        padding: 50px 0 80px 0;
        font-family: 'font_regular', sans-serif;
        color: var(--regular-text-color);

        &__image {
            width: 248px;
            height: 252px;
            margin-bottom: 40px;
        }

        &__title {
            font-size: 26px;
        }

        &__additional-text {
            font-size: 16px;
            max-width: 500px;
            text-align: center;
            margin-top: 16px;
        }
    }

    @media screen and (max-width: 640px) {

        .estimation-container {
            padding: 28px 20px 28px 20px;
        }

        .column-2,
        .column-3,
        .column-4,
        .column-5 {
            display: none;
        }

        .column-1 {
            width: 70%;
        }

        .column-6 {
            width: 30%;
        }
    }
</style>
