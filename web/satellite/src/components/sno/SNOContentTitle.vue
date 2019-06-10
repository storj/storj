<template>
	<div class="title">
		<div class="title__name">
			<h1>Your Storage Node Stats</h1>
			<p class="title__name__info">Current period: <b>June</b></p>
		</div>
		<div class="title__info">
			<svg width="18" height="18" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
				<path d="M9 0.5C13.6942 0.5 17.5 4.3058 17.5 9C17.5 13.6942 13.6942 17.5 9 17.5C4.3058 17.5 0.5 13.6942 0.5 9C0.5 4.3058 4.3058 0.5 9 0.5Z" fill="#00CE7D" stroke="#F4F6F9"/>
				<path fill-rule="evenodd" clip-rule="evenodd" d="M4.35717 9.90354C3.30671 8.7687 5.03287 7.1697 6.08406 8.30604L7.78632 10.144L11.8784 5.31912C12.8797 4.13577 14.6803 5.66083 13.6792 6.84279L8.7531 12.6514C8.28834 13.1977 7.4706 13.2659 6.96364 12.7182L4.35717 9.90354Z" fill="#F4F6F9"/>
			</svg>
			<svg width="27" height="20" viewBox="0 0 27 20" fill="none" xmlns="http://www.w3.org/2000/svg">
				<path d="M1.0896 11.5265V9.95801H25.9184V11.5265H1.0896Z" fill="#535F77"/>
				<path fill-rule="evenodd" clip-rule="evenodd" d="M26.681 14.1731V10.792C26.681 10.4148 26.5786 10.0448 26.3855 9.72357L22.539 3.34291C21.5179 1.64753 19.7154 0.615967 17.7755 0.615967H8.60176C6.51114 0.615967 4.59434 1.81197 3.62896 3.71888L0.549768 9.80582C0.403324 10.0939 0.326904 10.4152 0.326904 10.7422V14.1731C0.326904 17.3561 2.83589 19.9361 5.93124 19.9361H21.0767C24.172 19.9361 26.681 17.3561 26.681 14.1731ZM25.0886 10.5492C25.1323 10.622 25.1557 10.7064 25.1557 10.792V14.1731C25.1557 16.4898 23.3296 18.3676 21.0767 18.3676H5.93124C3.6783 18.3676 1.85222 16.4898 1.85222 14.1731V10.7422C1.85222 10.6677 1.86933 10.5958 1.90222 10.5311L4.98203 4.44295C5.68465 3.05506 7.07991 2.18449 8.60176 2.18449H17.7755C19.1875 2.18449 20.4993 2.93527 21.2424 4.16907L25.0886 10.5492Z" fill="#535F77"/>
				<path d="M22.3542 14.4712C22.7754 14.4712 23.1169 14.8223 23.1169 15.2555C23.1169 15.6886 22.7754 16.0397 22.3542 16.0397H17.9223C17.5011 16.0397 17.1597 15.6886 17.1597 15.2555C17.1597 14.8223 17.5011 14.4712 17.9223 14.4712H22.3542Z" fill="#535F77"/>
			</svg>
			<p class="online-status"><b>{{node.status}}</b></p>
			<p><b>Node Version</b></p>
			<p class="version">{{node.version}}</p>
			<svg width="18" height="18" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
				<path d="M9 0.5C13.6942 0.5 17.5 4.3058 17.5 9C17.5 13.6942 13.6942 17.5 9 17.5C4.3058 17.5 0.5 13.6942 0.5 9C0.5 4.3058 4.3058 0.5 9 0.5Z" fill="#00CE7D" stroke="#F4F6F9"/>
				<path fill-rule="evenodd" clip-rule="evenodd" d="M4.35717 9.90354C3.30671 8.7687 5.03287 7.1697 6.08406 8.30604L7.78632 10.144L11.8784 5.31912C12.8797 4.13577 14.6803 5.66083 13.6792 6.84279L8.7531 12.6514C8.28834 13.1977 7.4706 13.2659 6.96364 12.7182L4.35717 9.90354Z" fill="#F4F6F9"/>
			</svg>
			<div class="title__info__satellite-selection-toggle-container">
				<p><b>Satellite</b></p>
				<p>{{satellite.selected}}</p>
				<svg width="8" height="4" viewBox="0 0 8 4" fill="none" xmlns="http://www.w3.org/2000/svg">
					<path d="M4.66343 0.268927C4.29704 -0.0911446 3.70059 -0.0881362 3.33763 0.268927L0.20204 3.34916C-0.16435 3.70848 -0.0186405 4 0.52753 4L7.47359 4C8.01976 4 8.16292 3.70548 7.79908 3.34916L4.66343 0.268927Z" fill="#535F77"/>
				</svg>
				<SatelliteSelectionDropdown v-if="true"/>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import SatelliteSelectionDropdown from './SatelliteSelectionDropdown.vue';

    @Component ({
        computed: {
            node: function ()  {
                return this.$store.state.fakeModule.node;
            },

            satellite: function () {
	            return this.$store.state.fakeModule.satellite;
            },
        },
	    components: {
            SatelliteSelectionDropdown,
        },
    })

    export default class SNOContentTitle extends Vue {
    }
</script>

<style lang="scss">
	.title {
		display: flex;
		width: 100%;
		justify-content: space-between;
		align-items: center;
		font-family: 'font_regular';
		padding: 0 0 12px 0;

		&__name {
			margin-right: 45px;

			h1 {
				margin: 0;
				font-size: 24px;
			}

			p {
				margin: 0;
				font-size: 12px;
			}
		}

		&__info {
			display: flex;
			justify-content: space-between;
			align-items: center;
			font-size: 12px;

			.online-status {
				margin: 0 20px 0 5px;
			}

			.version {
				margin: 0 5px 0 5px;
			}

			&__satellite-selection-toggle-container {
				width: 168px;
				height: 44px;
				display: flex;
				justify-content: flex-start;
				align-items: center;
				background-color: #FFFFFF;
				border: 1px solid #E8E8E8;
				border-radius: 12px;
				padding: 0 14px 0 14px;
				position: relative;
				font-size: 14px;
				margin-left: 24px;

				b {
					margin-right: 3px;
				}

				svg {
					position: absolute;
					right: 14px;
				}
			}
		}
	}
</style>
