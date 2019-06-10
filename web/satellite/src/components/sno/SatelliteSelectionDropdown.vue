<template>
	<div class="satellite-selection-choice-container" id="satelliteDropdown">
		<div class="satellite-selection-overflow-container">
			<!-- loop for rendering satellites -->
			<!-- TODO: add selection logic onclick -->
			<div class="satellite-selection-overflow-container__satellite-choice" v-for="satellite in satelliteList" v-bind:key="satellite.id" >
				<p v-bind:class="[satellite.isSelected ? 'satellite-selection-overflow-container__project-choice--selected' : 'project-selection-overflow-container__project-choice--unselected']">{{satellite.name}}</p>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';

    @Component ({
        computed: {
            satelliteList: function () {
                return this.$store.state.fakeModule.satellite.list;
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
				line-height: 20px;
				font-family: 'font_regular';
			}

			&:hover {
				background-color: #EBECF0;
			}

			&--selected {
				font-family: 'font_bold';
			}

			&--unselected {
				font-family: 'font_regular';
			}
		}
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
