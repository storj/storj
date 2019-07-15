// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div class="info-area">
		<p class="info-area__title">Utilization & Remaining</p>
		<div class="info-area__chart-area">
			<div class="chart-container">
				<p class="chart-container__title">Bandwidth Used This Month</p>
				<p class="chart-container__amount"><b>{{bandwidth.used}}</b></p>
				<div class="chart-container__chart">
					<BandwidthChart />
				</div>
			</div>
			<div class="chart-container">
				<p class="chart-container__title">Disk Space Used This Month</p>
				<p class="chart-container__amount"><b>{{diskSpace.used}}</b></p>
				<div class="chart-container__chart">
					<DiskSpaceChart />
				</div>
			</div>
		</div>
		<div class="info-area__remaining-space-area">
			<BarInfoContainer label="Bandwidth Remaining" :amount="bandwidth.remaining"
							  infoText="of bandwidth left" :currentBarAmount="bandwidth.used" :maxBarAmount="bandwidth.available" />
			<BarInfoContainer label="Disk Space Remaining" :amount="diskSpace.remaining"
							  infoText="of disk space left" :currentBarAmount="diskSpace.used" :maxBarAmount="diskSpace.available" />
		</div>
		<div v-if="isSatelliteSelected">
			<p class="info-area__title">Uptime & Audit Checks by Satellite</p>
			<div class="info-area__checks-area">
				<ChecksAreaContainer label="Uptime Checks" :amount="checks.uptime" infoText="Uptime checks occur to make sure  your node is still online. This is the percentage of uptime checks you’ve passed."/>
				<ChecksAreaContainer label="Audit Checks" :amount="checks.audit" infoText="Audit checks occur to make sure the file data sent to your storage node is still there and intact. This is the percentage of audit checks you’ve passed."/>
			</div>
		</div>
		<p class="info-area__title">Payout</p>
		<PayoutContainer label="STORJ Wallet Address" :walletAddress="wallet" />
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
	import BandwidthChart from '@/components/BandwidthChart.vue';
	import DiskSpaceChart from '@/components/DiskSpaceChart.vue';
	import BarInfoContainer from '@/components/BarInfoContainer.vue';
	import ChecksAreaContainer from '@/components/ChecksAreaContainer.vue';
	import PayoutContainer from '@/components/PayoutContainer.vue';

    @Component ({
		components: {
			BandwidthChart,
			DiskSpaceChart,
			BarInfoContainer,
			ChecksAreaContainer,
			PayoutContainer,
		},

		computed: {
			wallet: function ()  {
				return this.$store.state.nodeModule.node.wallet;
			},

			bandwidth: function () {
				return this.$store.state.nodeModule.bandwidth;
			},

			diskSpace: function () {
				return this.$store.state.nodeModule.diskSpace;
			},

			checks: function () {
				return this.$store.state.nodeModule.checks;
			},

            isSatelliteSelected: function (): boolean {
                return !!this.$store.state.nodeModule.selectedSatellite;
            }
		},
    })

    export default class SNOContentFilling extends Vue {
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
		font-family: 'font_regular';

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
