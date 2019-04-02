// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div class="usage-report-container">
		<div class="usage-report-container__navigation">
			<div class="usage-report-container__navigation__button" @click="onBackClick">
				<svg width="19" height="19" viewBox="0 0 19 19" fill="none" xmlns="http://www.w3.org/2000/svg">
					<path fill-rule="evenodd" clip-rule="evenodd" d="M10.5607 0.43934C11.1464 1.02513 11.1464 1.97487 10.5607 2.56066L5.12132 8H17.5C18.3284 8 19 8.67157 19 9.5C19 10.3284 18.3284 11 17.5 11H5.12132L10.5607 16.4393C11.1464 17.0251 11.1464 17.9749 10.5607 18.5607C9.97487 19.1464 9.02513 19.1464 8.43934 18.5607L0.43934 10.5607C-0.146447 9.97487 -0.146447 9.02513 0.43934 8.43934L8.43934 0.43934C9.02513 -0.146447 9.97487 -0.146447 10.5607 0.43934Z" fill="#384B65"/>
				</svg>
				<p>Back to Project Details</p>
			</div>
		</div>
		<div class="usage-report-container__header">
			<p>Usage Report</p>
			<div class="usage-report-container__header__options-area">
				<div class="usage-report-container__header__options-area__option active" @click.prevent="onCurrentRollupClick">
					<p>Current roll up period</p>
				</div>
				<div class="usage-report-container__header__options-area__option" @click.prevent="onPreviousRollupClick">
					<p>Previous Roll Up Period</p>
				</div>
				<div class="usage-report-container__header__options-area__option" @click.prevent.self="onCustomDateClick">
					<p @click.prevent.self="onCustomDateClick">Custom Date Range</p>
					<Datepicker ref="datePicker" :date="startTime" @change="getDates"/>
					<svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" @click.prevent.self="onCustomDateClick">
						<path d="M16.3213 2.28026H14.8009V1.50058C14.8009 1.10058 14.4806 0.760742 14.0611 0.760742C13.6611 0.760742 13.3213 1.08106 13.3213 1.50058V2.28026H6.66106V1.50058C6.66106 1.10058 6.34074 0.760742 5.92122 0.760742C5.5009 0.760742 5.2009 1.10058 5.2009 1.50058V2.28026H3.68058C1.92042 2.28026 0.500977 3.70058 0.500977 5.45986V16.0599C0.500977 17.82 1.9213 19.2395 3.68058 19.2395H16.3204C18.0805 19.2395 19.5 17.8191 19.5 16.0599V5.45986C19.5008 3.70048 18.0804 2.28026 16.321 2.28026H16.3213ZM3.68066 3.74042H5.20098V4.5201C5.20098 4.9201 5.5213 5.25994 5.94082 5.25994C6.36114 5.25994 6.68066 4.93962 6.68066 4.5201V3.74042H13.3603V4.5201C13.3603 4.9201 13.6806 5.25994 14.1001 5.25994C14.5001 5.25994 14.8399 4.93962 14.8399 4.5201V3.74042H16.3603C17.3001 3.74042 18.0806 4.50058 18.0806 5.46074V7.06074H1.96098V5.46074C1.96098 4.5209 2.74066 3.74042 3.68052 3.74042H3.68066ZM9.62126 14.2006H10.4009C11.0213 14.2006 11.5213 14.7006 11.5213 15.3209C11.5213 15.9413 11.0213 16.4413 10.4009 16.4413H9.62126C9.00094 16.4413 8.50094 15.9413 8.50094 15.3209C8.50094 14.7006 9.00094 14.2006 9.62126 14.2006ZM8.50094 10.8404C8.50094 10.2201 9.00094 9.7201 9.62126 9.7201L10.4009 9.72088C11.0213 9.72088 11.5213 10.2209 11.5213 10.8412C11.5213 11.4615 11.0213 11.9615 10.4009 11.9615H9.62126C9.00094 11.9607 8.50094 11.4607 8.50094 10.8404V10.8404ZM14.8407 14.2006H15.6204C16.2407 14.2006 16.7407 14.7006 16.7407 15.3209C16.7407 15.9413 16.2407 16.4413 15.6204 16.4413H14.8407C14.2204 16.4413 13.7204 15.9413 13.7204 15.3209C13.7212 14.7006 14.2212 14.2006 14.8407 14.2006ZM13.7212 10.8404C13.7212 10.2201 14.2212 9.7201 14.8415 9.7201H15.6212C16.2415 9.7201 16.7415 10.2201 16.7415 10.8404C16.7415 11.4607 16.2415 11.9607 15.6212 11.9607H14.8415C14.2212 11.9607 13.7212 11.4607 13.7212 10.8404ZM6.2806 10.8404C6.2806 11.4607 5.7806 11.9607 5.16028 11.9607H4.3806C3.76028 11.9607 3.26028 11.4607 3.26028 10.8404C3.26028 10.2201 3.76028 9.7201 4.3806 9.7201H5.16028C5.7806 9.72088 6.2806 10.2209 6.2806 10.8404ZM4.3806 14.2006H5.16028C5.7806 14.2006 6.2806 14.7006 6.2806 15.3209C6.2806 15.9413 5.7806 16.4413 5.16028 16.4413H4.3806C3.76028 16.4413 3.26028 15.9413 3.26028 15.3209C3.26106 14.7006 3.76106 14.2006 4.3806 14.2006Z" fill="#2683FF"/>
					</svg>
				</div>
			</div>
		</div>
		<div class="usage-report-container__main-area">
			<div class="usage-report-container__main-area__info-area">
				<div class="usage-report-container__main-area__info-area__item">
					<h1>Storage GB/H</h1>
					<h2>200.23</h2>
				</div>
				<div class="usage-report-container__main-area__info-area__item">
					<h1>Egress GB/H</h1>
					<h2>944.23</h2>
				</div>
				<div class="usage-report-container__main-area__info-area__item">
					<h1>Objects Count/H</h1>
					<h2>30 223</h2>
				</div>
			</div>
			<div class="usage-report-container__main-area__footer">
				<p>Current Roll Up Period <b>{{dateRange.startDate.toLocaleDateString("en-US")}}</b> to <b>{{dateRange.endDate.toLocaleDateString("en-US")}}</b></p>
				<div class="usage-report-container__main-area__footer__report-area">
					<p>Download Advanced Report</p>
					<svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg" @click.prevent="onReportClick">
						<rect width="40" height="40" rx="4" fill="#E2ECF7"/>
						<path d="M25.6491 19.2809L21.2192 23.5281C20.891 23.8427 20.3988 24 20.0707 24C19.7425 24 19.2503 23.8427 19.0862 23.5281L14.4922 19.2809C13.8359 18.6517 13.8359 17.8652 14.4922 17.236C14.8204 16.9213 15.1485 16.9213 15.6407 16.9213C15.9689 16.9213 16.4611 17.0787 16.6252 17.3933L18.594 19.1236L18.594 11.4157C18.594 10.6292 19.2503 10 20.0707 10C20.891 10 21.5473 10.6292 21.5473 11.4157L21.5473 19.1236L23.5162 17.236C23.6803 16.9213 24.1725 16.9213 24.5006 16.9213C24.8288 16.9213 25.321 17.0787 25.4851 17.3933C26.1414 17.8652 26.1414 18.809 25.6491 19.2809Z" fill="#2683FF"/>
						<rect x="11" y="28" width="18" height="2" rx="1" fill="#2683FF"/>
						<defs>
							<clipPath id="clip0">
								<rect width="22" height="22" fill="white" transform="translate(10 10)"/>
							</clipPath>
						</defs>
					</svg>
				</div>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import ROUTES from '@/utils/constants/routerConstants';
    import Datepicker from '@/components/project/DatePicker.vue';

    @Component(
        {
	        data: function () {
	            const currentDate = new Date();
	            const previousDate = new Date();
	            previousDate.setMonth(currentDate.getMonth() - 1);

	            return {
                    startTime: {
                        time: '',
                    },
                    dateRange: {
                        startDate: previousDate,
	                    endDate: currentDate,
                    }
	            };
	        },
            components: {
                Datepicker,
            },
            methods: {
                getDates: function(datesArray: string[]): void {
                    const firstDate = new Date(datesArray[0]);
                    const secondDate = new Date(datesArray[1]);
                    const isInverted = firstDate > secondDate;
					this.$data.dateRange.startDate = isInverted ? secondDate : firstDate;
                    this.$data.dateRange.endDate = isInverted ? firstDate : secondDate;
                },
                onBackClick: function (): void {
                    this.$router.push(ROUTES.PROJECT_DETAILS);
                },
	            onCurrentRollupClick: function (event: any) {
                    const currentDate = new Date();
                    const previousDate = new Date();
                    previousDate.setMonth(currentDate.getMonth() - 1);

                    this.$data.dateRange.startDate = previousDate;
                    this.$data.dateRange.endDate = currentDate;
	                (this as any).onButtonClickAction(event);
	            },
                onPreviousRollupClick: function (event: any) {
                    const currentDate = new Date();
                    const previousDate = new Date();
                    currentDate.setMonth(currentDate.getMonth() - 1);
                    previousDate.setMonth(currentDate.getMonth() - 1);

                    this.$data.dateRange.startDate = previousDate;
                    this.$data.dateRange.endDate = currentDate;
                    (this as any).onButtonClickAction(event);
                },
                onCustomDateClick: function (event: any) {
	                (this as any).$refs.datePicker.showCheck();
	                (this as any).onButtonClickAction(event);
                },
                onButtonClickAction: function (event: any) {
                    let eventTarget = event.target;

                    if (eventTarget.children.length === 0) {
                        eventTarget = eventTarget.parentNode;
                    }

                    if (eventTarget.classList.contains('active')) {
                        return;
                    }

                    (this as any).changeActiveClass(eventTarget);
                },
                changeActiveClass: function (target: any): void {
                    [...document.querySelectorAll('.usage-report-container__header__options-area__option')].forEach(option => {
                        option.classList.remove('active');
                    });

                    target.classList.add('active');
                },
                onReportClick: function (): void {
                    let route = this.$router.resolve(ROUTES.REPORT_TABLE);
                    window.open(route.href, '_blank');
                },
            },
        }
    )

    export default class UsageReport extends Vue {}
</script>

<style scoped lang="scss">
	.usage-report-container {
		padding: 55px 60px 55px 60px;
		position: relative;

		&__navigation {
			display: flex;
			flex-direction: row;
			align-items: center;
			justify-content: flex-start;

			&__button {
				display: flex;
				flex-direction: row;
				align-items: center;
				justify-content: flex-start;
				cursor: pointer;

				p {
					font-family: 'font_medium';
					font-size: 18px;
					line-height: 24px;
					color: #354049;
					margin-left: 27px;
				}
			}
		}

		&__header {
			display: flex;
			flex-direction: row;
			align-items: center;
			justify-content: space-between;
			height: 56px;
			margin-top: 17px;

			p {
				font-family: 'font_bold';
				font-size: 32px;
				line-height: 39px;
				color: #354049;
			}

			&__options-area {
				display: flex;
				flex-direction: row;
				align-items: center;
				justify-content: space-between;
				height: 100%;

				&__option {
					display: flex;
					flex-direction: row;
					align-items: center;
					justify-content: center;
					width: 271px;
					height: 100%;
					background-color: #FFFFFF;
					border: solid 1px #F2F2F2;
					border-radius: 6px;
					cursor: pointer;
					margin-left: 20px;

					p {
						font-family: 'font_medium';
						font-size: 16px;
						line-height: 23px;
						color: #354049;
					}

					svg {
						margin-left: 10px;
					}

					&.active {
						background-color: #2683FF;

						p {
							color: #FFFFFF;
						}

						svg {
							path {
								fill: #ffffff !important;
							}
						}
					}
				}
			}
		}

		&__main-area {
			display: flex;
			flex-direction: column;
			margin-top: 33px;

			&__info-area {
				display: flex;
				flex-direction: row;
				align-items: center;
				justify-content: space-between;
				height: 40vh;

				&__item {
					max-width: 454px;
					max-height: 393px;
					width: 100%;
					height: 100%;
					background-color: #ffffff;
					display: flex;
					flex-direction: column;
					align-items: center;
					justify-content: center;
					border-radius: 6px;
					margin-right: 20px;

					&:last-child {
						margin-right: 0;
					}

					h1 {
						font-family: 'font_regular';
						font-size: 18px;
						line-height: 24px;
						color: #AFB7C1;
						margin-block-start: 0em;
						margin-block-end: 0em;
					}

					h2 {
						margin-top: 22px;
						font-family: 'font_medium';
						font-size: 44px;
						line-height: 67px;
						color: #354049;
						margin-block-start: 0.5em;
						margin-block-end: 0em;
					}
				}
			}

			&__footer {
				padding: 0 32px;
				height: 86px;
				margin-top: 24px;
				background-color: #ffffff;
				border-radius: 6px;
				display: flex;
				flex-direction: row;
				align-items: center;
				justify-content: space-between;

				p {
					font-family: 'font_regular';
					font-size: 20px;
					line-height: 27px;
					color: #AFB7C1;

					b {
						font-family: 'font_medium';
						color: #354049;
					}
				}

				&__report-area {
					display: flex;
					flex-direction: row;
					align-items: center;
					justify-content: space-between;

					p {
						font-family: 'font_medium';
						font-weight: bold;
						font-size: 20px;
						line-height: 29px;
						color: #354049;
						margin-right: 30px;
					}

					svg {
						cursor: pointer;
					}
				}
			}
		}
	}

	@media screen and (max-width: 1600px) {
		.usage-report-container {
			&__navigation {
				p {
					font-size: 16px;
					line-height: 21px;
				}
			}

			&__header {
				p {
					font-size: 24px;
					line-height: 29px;
				}
				&__options-area {
					&__option {
						width: 236px;
						p {
							font-size: 16px;
							line-height: 23px;
						}
					}
				}
			}

			&__main-area {
				&__info-area {
					&__item {
						h1 {
							font-size: 14px;
							line-height: 19px;
						}
						h2 {
							font-size: 36px;
							line-height: 54px;
						}
					}
				}

				&__footer {
					p {
						font-size: 16px;
						line-height: 21px;
					}
					&__report-area {

						p {
							font-size: 16px;
							line-height: 23px;
						}
					}
				}
			}
		}
	}

	@media screen and (max-width: 1350px) {
		.usage-report-container {
			&__header {
				&__options-area {
					&__option {
						width: 200px;
						p {
							font-size: 14px;
							line-height: 19px;
						}
						&:last-child {
							width: 60px;
							p {
								display: none;
							}
							svg {
								margin: 0;
							}
						}
					}
				}
			}
		}
	}
</style>
