// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="estimation-container">
        <div class="estimation-container__header">
            <p class="estimation-container__header__title">Info & Estimation</p>
            <EstimationPeriodDropdown />
        </div>
        <div class="estimation-container__divider"></div>
        <div>
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
            <div v-for="item in data" :key="item.payout" class="estimation-table-container__info-area">
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
            <div class="estimation-table-container__held-area">
                <p class="estimation-table-container__held-area__text">{{ heldInfo.surgePercent }}% Held back</p>
                <p class="estimation-table-container__held-area__text">-{{ held | centsToDollars }}</p>
            </div>
            <div class="estimation-table-container__total-area">
                <div class="column justify-start column-1">
                    <p class="estimation-table-container__total-area__text">TOTAL</p>
                </div>
                <div class="column justify-start column-2"></div>
                <div class="column justify-start column-3"></div>
                <div class="column justify-start column-4">
                    <p class="estimation-table-container__total-area__text">{{ totalDiskSpace }}m</p>
                </div>
                <div class="column justify-start column-5">
                    <p class="estimation-table-container__total-area__text">{{ totalBandwidth }}</p>
                </div>
                <div class="column justify-end column-6">
                    <p class="estimation-table-container__total-area__text">{{ totalPayout | centsToDollars }}</p>
                </div>
            </div>
        </div>
        <div class="estimation-container__payout-area" v-if="isCurrentPeriod">
            <div class="estimation-container__payout-area__left-area">
                <p class="title-text">Estimated Payout</p>
                <p class="additional-text">At the end of the month if the load keeps the same for the rest of the month.</p>
            </div>
            <div class="estimation-container__payout-area__right-area">
                <p class="title-text">{{ estimatedPayout | centsToDollars }}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue, Watch } from 'vue-property-decorator';

import EstimationPeriodDropdown from '@/app/components/payments/EstimationPeriodDropdown.vue';

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
    public data: EstimationTableRow[] = [];
    private BANDWIDTH_DOWNLOAD_PRICE_PER_TB = 2000;
    private BANDWIDTH_REPAIR_PRICE_PER_TB = 1000;
    private DISK_SPACE_PRICE_PER_TB = 150;

    /**
     * Indicates if current month selected.
     */
    public get isCurrentPeriod(): boolean {
        const end = this.$store.state.payoutModule.periodRange.end;
        const isCurrentMonthSelected = end.year === this.now.getUTCFullYear() && end.month === this.now.getUTCMonth();

        return !this.$store.state.payoutModule.periodRange.start && isCurrentMonthSelected;
    }

    /**
     * Returns estimated payout depends on bandwidth and disk space in current month.
     */
    @Watch('totalPayout')
    public get estimatedPayout(): number {
        const nodeStart = this.$store.state.node.info.startedAt;
        const daysInCurrentMonth: number = new Date(this.now.getFullYear(), this.now.getMonth() + 1, 0).getDate();

        if (
            nodeStart.getUTCFullYear() === this.now.getUTCFullYear()
            && nodeStart.getUTCMonth() === this.now.getUTCMonth()
        ) {
            const remainingDays = daysInCurrentMonth - this.now.getUTCDate();
            const daysOnline = daysInCurrentMonth - nodeStart.getUTCDate();

            return (this.totalPayout / daysOnline) * (daysOnline + remainingDays);
        }

        return (this.totalPayout / this.now.getUTCDate()) * daysInCurrentMonth;
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
    @Watch('isCurrentPeriod')
    @Watch('heldInfo')
    public get held(): number {
        if (this.isCurrentPeriod) {
            return (this.currentBandwidthDownload * this.BANDWIDTH_DOWNLOAD_PRICE_PER_TB
                + this.currentBandwidthAuditAndRepair * this.BANDWIDTH_REPAIR_PRICE_PER_TB
                + this.currentDiskSpace * this.DISK_SPACE_PRICE_PER_TB) / TB * this.heldInfo.surgePercent / 100;
        }

        return this.heldInfo.held;
    }

    /**
     * Returns calculated or stored total payout by selected period.
     */
    @Watch('isCurrentPeriod')
    @Watch('heldInfo')
    public get totalPayout(): number {
        if (this.isCurrentPeriod) {
            return (this.currentBandwidthDownload * this.BANDWIDTH_DOWNLOAD_PRICE_PER_TB
                + this.currentBandwidthAuditAndRepair * this.BANDWIDTH_REPAIR_PRICE_PER_TB
                + this.currentDiskSpace * this.DISK_SPACE_PRICE_PER_TB) / TB
                - this.held;
        }

        return this.$store.getters.totalPeriodPayout;
    }

    /**
     * Returns calculated or stored total used disk space by selected period.
     */
    @Watch('isCurrentPeriod')
    @Watch('heldInfo')
    public get totalDiskSpace(): string {
        if (this.isCurrentPeriod) {
            return formatBytes(this.currentDiskSpace);
        }

        return formatBytes(this.heldInfo.usageAtRest);
    }

    /**
     * Returns calculated or stored total used bandwidth by selected period.
     */
    @Watch('isCurrentPeriod')
    @Watch('heldInfo')
    public get totalBandwidth(): string {
        if (this.isCurrentPeriod) {
            return formatBytes(this.currentBandwidthAuditAndRepair + this.currentBandwidthDownload);
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
     * Lifecycle hook after initial render.
     * Builds estimated payout table.
     */
    public mounted(): void {
        this.buildTable();
    }

    /**
     * Builds estimated payout table depends on selected period.
     */
    @Watch('heldInfo')
    @Watch('isCurrentPeriod')
    private buildTable(): void {
        if (!this.isCurrentPeriod) {
            this.data = [
                new EstimationTableRow('Download', 'Egress', `$${this.BANDWIDTH_DOWNLOAD_PRICE_PER_TB / 100} / TB`, '--', formatBytes(this.heldInfo.usageGet), this.heldInfo.compGet),
                new EstimationTableRow('Repair & Audit', 'Egress', `$${this.BANDWIDTH_REPAIR_PRICE_PER_TB / 100} / TB`, '--', formatBytes(this.heldInfo.usageGetRepair + this.heldInfo.usageGetAudit), this.heldInfo.compGetRepair + this.heldInfo.compGetAudit),
                new EstimationTableRow('Disk Average Month', 'Storage', `$${this.DISK_SPACE_PRICE_PER_TB / 100} / TBm`, formatBytes(this.heldInfo.usageAtRest) + 'm', '--', this.heldInfo.compAtRest),
            ];

            return;
        }

        this.data = [
            new EstimationTableRow(
                'Download',
                'Egress',
                `$${this.BANDWIDTH_DOWNLOAD_PRICE_PER_TB / 100} / TB`,
                '--',
                formatBytes(this.currentBandwidthDownload),
                this.currentBandwidthDownload * this.BANDWIDTH_DOWNLOAD_PRICE_PER_TB / TB,
            ),
            new EstimationTableRow(
                'Repair & Audit',
                'Egress',
                `$${this.BANDWIDTH_REPAIR_PRICE_PER_TB / 100} / TB`,
                '--',
                formatBytes(this.currentBandwidthAuditAndRepair),
                this.currentBandwidthAuditAndRepair * this.BANDWIDTH_REPAIR_PRICE_PER_TB / TB,
            ),
            new EstimationTableRow(
                'Disk Average Month',
                'Storage',
                `$${this.DISK_SPACE_PRICE_PER_TB / 100} / TBm`,
                this.totalDiskSpace + 'm',
                '--',
                this.currentDiskSpace * this.DISK_SPACE_PRICE_PER_TB / TB,
            ),
        ];
    }
}
</script>

<style scoped lang="scss">
    .estimation-container {
        display: flex;
        flex-direction: column;
        padding: 28px 40px 28px 40px;
        background: #fff;
        border: 1px solid #eaeaea;
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
                color: #535f77;
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
        color: #535f77;
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
            background: #f9fafc;

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
                color: #535f77;
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
                color: #535f77;
            }
        }

        &__total-area {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 56px;
            background: rgba(0, 117, 255, 0.05);

            &__text {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                color: #535f77;
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
</style>
