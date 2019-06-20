// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div class="info-area">
		<p class="info-area__title">Utilization & Remaining</p>
		<div class="info-area__chart-area">
			<div class="chart-container">
				<p class="chart-container__title">Bandwidth Used This Month</p>
				<p class="chart-container__amount"><b>{{bandwidth.used}}</b>GB</p>
				<div class="chart-container__chart">
					<BandwidthChart />
				</div>
			</div>
			<div class="chart-container">
				<p class="chart-container__title">Disk Space Used This Month</p>
				<p class="chart-container__amount"><b>{{diskSpace.used}}</b>GB</p>
				<div class="chart-container__chart">
					<DiskSpaceChart />
				</div>
			</div>
		</div>
		<div class="info-area__remaining-space-area">
			<BarInfoContainer label="Bandwidth Remaining" :amount="bandwidth.remaining"
							  infoText="36% of bandwidth left" currentBarAmount="766" maxBarAmount="1000" />
			<BarInfoContainer label="Disk Space Remaining" :amount="diskSpace.remaining"
							  infoText="10% of bandwidth left" currentBarAmount="456" maxBarAmount="1000" />
		</div>
		<p class="info-area__title">Uptime & Audit Checks by Satellite</p>
		<div class="info-area__checks-area">
			<ChecksAreaContainer label="Uptime Checks" :amount="checks.uptime" infoText="text place"/>
			<ChecksAreaContainer label="Audit Checks" :amount="checks.audit" infoText="text place"/>
		</div>
		<p class="info-area__title">Payout</p>
		<PayoutContainer label="STORJ Wallet Address" :walletAddress="wallet.address" />
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
				return this.$store.state.wallet;
			},

			bandwidth: function () {
				return this.$store.state.bandwidth;
			},

			diskSpace: function () {
				return this.$store.state.diskSpace;
			},

			checks: function () {
				return this.$store.state.checks;
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
