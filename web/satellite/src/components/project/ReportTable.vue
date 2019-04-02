// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div>
		<table class="blueTable">
			<thead>
				<tr>
					<th>Bucket Name</th>
					<th>Roll Up Period</th>
					<th>Network Stored Data, GBh</th>
					<th colspan="3">Egress, GB</th>
					<th colspan="2">Objects, count*hours</th>
					<th>Metadata Size, GBh</th>
				</tr>
				<tr>
					<th></th>
					<th></th>
					<th></th>
					<th>Repair</th>
					<th>Get</th>
					<th>Audit</th>
					<th>Inline Segments</th>
					<th>Remote segments</th>
					<th></th>
				</tr>
			</thead>
			<tbody>
				<tr>
					<td class="tg-0lax">My Data Bucket 1</td>
					<td class="tg-0lax">01.02.2019-01.28.2019</td>
					<td class="tg-0lax">800</td>
					<td class="tg-0lax">10</td>
					<td class="tg-0lax">400</td>
					<td class="tg-0lax">20</td>
					<td class="tg-0lax">1000</td>
					<td class="tg-0lax">700</td>
					<td class="tg-0lax">0.1</td>
				</tr>
				<tr>
					<td class="tg-0lax">My Data Bucket 2</td>
					<td class="tg-0lax">01.02.2019-01.28.2019</td>
					<td class="tg-0lax">700</td>
					<td class="tg-0lax">10</td>
					<td class="tg-0lax">400</td>
					<td class="tg-0lax">20</td>
					<td class="tg-0lax">1000</td>
					<td class="tg-0lax">700</td>
					<td class="tg-0lax">0.2</td>
				</tr>
				<tr>
					<td class="tg-0lax">My Data Bucket 3</td>
					<td class="tg-0lax">01.02.2019-01.28.2019</td>
					<td class="tg-0lax">600</td>
					<td class="tg-0lax">10</td>
					<td class="tg-0lax">400</td>
					<td class="tg-0lax">20</td>
					<td class="tg-0lax">1000</td>
					<td class="tg-0lax">700</td>
					<td class="tg-0lax">0.3</td>
				</tr>
				<tr>
					<td class="tg-0lax">My Data Bucket 4</td>
					<td class="tg-0lax">01.02.2019-01.28.2019</td>
					<td class="tg-0lax">550</td>
					<td class="tg-0lax">10</td>
					<td class="tg-0lax">400</td>
					<td class="tg-0lax">20</td>
					<td class="tg-0lax">1000</td>
					<td class="tg-0lax">700</td>
					<td class="tg-0lax">0.15</td>
				</tr>
				<tr>
					<td class="tg-0lax">My Data Bucket 5</td>
					<td class="tg-0lax">01.02.2019-01.28.2019</td>
					<td class="tg-0lax">500</td>
					<td class="tg-0lax">10</td>
					<td class="tg-0lax">400</td>
					<td class="tg-0lax">20</td>
					<td class="tg-0lax">1000</td>
					<td class="tg-0lax">700</td>
					<td class="tg-0lax">0.11</td>
				</tr>
				<tr>
					<td class="tg-0lax">My Data Bucket 6</td>
					<td class="tg-0lax">01.02.2019-01.28.2019</td>
					<td class="tg-0lax">1000</td>
					<td class="tg-0lax">10</td>
					<td class="tg-0lax">400</td>
					<td class="tg-0lax">20</td>
					<td class="tg-0lax">1000</td>
					<td class="tg-0lax">700</td>
					<td class="tg-0lax">0.13</td>
				</tr>
				<tr>
					<td class="tg-0lax">My Data Bucket 7</td>
					<td class="tg-0lax">01.02.2019-01.28.2019</td>
					<td class="tg-0lax">1200</td>
					<td class="tg-0lax">10</td>
					<td class="tg-0lax">400</td>
					<td class="tg-0lax">20</td>
					<td class="tg-0lax">1000</td>
					<td class="tg-0lax">700</td>
					<td class="tg-0lax">0.13</td>
				</tr>
				<tr>
					<td class="tg-0lax">My Data Bucket 8</td>
					<td class="tg-0lax">01.02.2019-01.28.2019</td>
					<td class="tg-0lax">1300</td>
					<td class="tg-0lax">10</td>
					<td class="tg-0lax">400</td>
					<td class="tg-0lax">20</td>
					<td class="tg-0lax">1000</td>
					<td class="tg-0lax">700</td>
					<td class="tg-0lax">0.13</td>
				</tr>
				<tr>
					<td class="tg-0lax">My Data Bucket 9</td>
					<td class="tg-0lax">01.02.2019-01.28.2019</td>
					<td class="tg-0lax">343</td>
					<td class="tg-0lax">10</td>
					<td class="tg-0lax">400</td>
					<td class="tg-0lax">20</td>
					<td class="tg-0lax">1000</td>
					<td class="tg-0lax">700</td>
					<td class="tg-0lax">0.13</td>
				</tr>
				<tr>
					<td class="tg-0lax">My Data Bucket 10</td>
					<td class="tg-0lax">01.02.2019-01.28.2019</td>
					<td class="tg-0lax">12345</td>
					<td class="tg-0lax">10</td>
					<td class="tg-0lax">400</td>
					<td class="tg-0lax">20</td>
					<td class="tg-0lax">1000</td>
					<td class="tg-0lax">700</td>
					<td class="tg-0lax">0.13</td>
				</tr>
			</tbody>
		</table>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';

    @Component(
        {}
    )

    export default class ReportTable extends Vue {}
</script>

<style scoped lang="scss">
	table.blueTable {
		border: 1px solid #1C6EA4;
		background-color: #EEEEEE;
		width: 100%;
		text-align: center;
		border-collapse: collapse;
	}
	table.blueTable td, table.blueTable th {
		border: 1px solid #AAAAAA;
		padding: 3px 2px;
	}
	table.blueTable tbody td {
		font-size: 13px;
	}
	table.blueTable tbody tr:nth-child(even) {
		background: #D0E4F5;
	}
	table.blueTable thead {
		background: #1C6EA4;
		background: -moz-linear-gradient(top, #5592bb 0%, #327cad 66%, #1C6EA4 100%);
		background: -webkit-linear-gradient(top, #5592bb 0%, #327cad 66%, #1C6EA4 100%);
		background: linear-gradient(to bottom, #5592bb 0%, #327cad 66%, #1C6EA4 100%);
		border-bottom: 2px solid #444444;
	}
	table.blueTable thead th {
		font-size: 15px;
		font-weight: bold;
		color: #FFFFFF;
		border-left: 2px solid #D0E4F5;
	}
	table.blueTable thead th:first-child {
		border-left: none;
	}
</style>