// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div class="satellite-selection-choice-container" id="satelliteDropdown">
		<div class="satellite-selection-overflow-container">
			<!-- loop for rendering satellites -->
			<!-- TODO: add selection logic onclick -->
			<div class="satellite-selection-overflow-container__satellite-choice"
				 v-for="satellite in satellites" v-bind:key="satellite"
				 @click.stop="onSatelliteClick(satellite)" >
				<p :class="{selected: satellite === selectedSatellite}">{{satellite}}</p>
			</div>
			<div class="satellite-selection-choice-container__all-satellites">
				<div class="satellite-selection-overflow-container__satellite-choice" @click.stop="onSatelliteClick(null)">
					<p :class="{selected: !selectedSatellite}">All Satellites</p>
				</div>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import { NODE_ACTIONS, APPSTATE_ACTIONS } from '@/utils/constants'

    @Component ({
		methods: {
			onSatelliteClick: async function (id: string): Promise<any> {
				id ?
					await this.$store.dispatch(NODE_ACTIONS.GET_NODE_INFO, `/api/dashboard/?satelliteId=${id}`)
					: await this.$store.dispatch(NODE_ACTIONS.GET_NODE_INFO, `/api/dashboard/}`);

				this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
				this.$store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, id);
			}
		},
        computed: {
            satellites: function () {
                return this.$store.state.nodeModule.satellites;
            },
			selectedSatellite: function () {
				return this.$store.state.nodeModule.selectedSatellite;
			}
        },
    })

    export default class SatelliteChoiceDropdown extends Vue {
    }
</script>

<style lang="scss">
	.satellite-selection-choice-container {
		position: absolute;
		top: 50px;
		right: 0;
		width: 224px;
		border-radius: 8px;
		padding: 7px 0 7px 0;
		box-shadow: 0 4px 4px rgba(0, 0, 0, 0.25);
		background-color: #FFFFFF;
		z-index: 1120;
	}
	.satellite-selection-overflow-container {
		position: relative;
		overflow-y: auto;
		overflow-x: hidden;
		height: auto;

		&__satellite-choice {
			display: flex;
			width: 186px;
			flex-direction: row;
			align-items: center;
			justify-content: flex-start;
			margin-left: 8px;
			border-radius: 12px;
			padding: 0 0 0 12px;

			p {
				font-size: 14px;
				line-height: 40px;
				font-family: 'font_regular';
			}

			&:hover {
				background-color: #EBECF0;
				cursor: pointer;
			}
		}

		&__all-satellites {
			padding: 0 0 0 12px;
		}
	}

	.selected {
		font-family: 'font_bold';
	}

	/* width */
	::-webkit-scrollbar {
		width: 4px;
	}

	/* Track */
	::-webkit-scrollbar-track {
		box-shadow: inset 0 0 5px #fff;
	}

	/* Handle */
	::-webkit-scrollbar-thumb {
		background: #AFB7C1;
		border-radius: 6px;
		height: 5px;
	}
</style>
