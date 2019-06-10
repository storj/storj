<template>
	<div class="info-area">
		<p class="info-area__title">Utilization & Remaining</p>
		<div class="info-area__chart-area">
			<div class="chart-container">
				<p class="chart-container__title">Bandwidth Used This Month</p>
				<p class="chart-container__amount"><b>165</b>GB</p>
				<div class="chart-container__chart">

				</div>
			</div>
			<div class="chart-container">
				<p class="chart-container__title">Disk Space Used This Month</p>
				<p class="chart-container__amount"><b>82</b>GB</p>
				<div class="chart-container__chart">

				</div>
			</div>
		</div>
		<div class="info-area__remaining-space-area">
			<div class="remaining-space-container">
				<p class="remaining-space-container__title">Bandwidth Remaining</p>
				<p class="remaining-space-container__amount"><b>234</b>GB</p>
				<div class="remaining-space-container__bar">

				</div>
			</div>
			<div class="remaining-space-container">
				<p class="remaining-space-container__title">Disk Space Remaining</p>
				<p class="remaining-space-container__amount"><b>544</b>GB</p>
				<div class="remaining-space-container__bar">

				</div>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';

    @Component ({

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
		font-family: 'font-regular';

		&__title {
			font-size: 18px;
			line-height: 40px;
		}

		&__chart-area,
		&__remaining-space-area	{
			display: flex;
			justify-content: space-between;
		}
	}

	.chart-container,
	.remaining-space-container {
		width: 361px;
		height: 257px;
		background-color: #FFFFFF;
		border: 1px solid #E9EFF4;
		border-radius: 11px;
		padding: 34px 0 39px 39px;
		margin-bottom: 32px;

		&__title {
			font-size: 14px;
		}

		&__amount {
			font-size: 32px;
		}
	}

	.remaining-space-container {
		height: 90px;
		display: flex;
		flex-direction: column;
		justify-content: space-between;
	}
</style>
