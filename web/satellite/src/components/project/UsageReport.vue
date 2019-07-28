// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="usage-report-container">
        <div class="usage-report-container__header">
            <p>Report</p>
        </div>
        <div class="usage-report-container__options-area">
            <div class="usage-report-container__options-area__option active" @click.prevent="onCurrentRollupClick">
                <p>Current Billing Period</p>
            </div>
            <div class="usage-report-container__options-area__option" @click.prevent="onPreviousRollupClick">
                <p>Previous Billing Period</p>
            </div>
            <div class="usage-report-container__options-area__option" @click.prevent.self="onCustomDateClick">
                <p @click.prevent.self="onCustomDateClick">Custom Date Range</p>
                <Datepicker ref="datePicker" :date="startTime" @change="getDates"/>
                <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" @click.prevent.self="onCustomDateClick">
                    <path d="M16.3213 2.28026H14.8009V1.50058C14.8009 1.10058 14.4806 0.760742 14.0611 0.760742C13.6611 0.760742 13.3213 1.08106 13.3213 1.50058V2.28026H6.66106V1.50058C6.66106 1.10058 6.34074 0.760742 5.92122 0.760742C5.5009 0.760742 5.2009 1.10058 5.2009 1.50058V2.28026H3.68058C1.92042 2.28026 0.500977 3.70058 0.500977 5.45986V16.0599C0.500977 17.82 1.9213 19.2395 3.68058 19.2395H16.3204C18.0805 19.2395 19.5 17.8191 19.5 16.0599V5.45986C19.5008 3.70048 18.0804 2.28026 16.321 2.28026H16.3213ZM3.68066 3.74042H5.20098V4.5201C5.20098 4.9201 5.5213 5.25994 5.94082 5.25994C6.36114 5.25994 6.68066 4.93962 6.68066 4.5201V3.74042H13.3603V4.5201C13.3603 4.9201 13.6806 5.25994 14.1001 5.25994C14.5001 5.25994 14.8399 4.93962 14.8399 4.5201V3.74042H16.3603C17.3001 3.74042 18.0806 4.50058 18.0806 5.46074V7.06074H1.96098V5.46074C1.96098 4.5209 2.74066 3.74042 3.68052 3.74042H3.68066ZM9.62126 14.2006H10.4009C11.0213 14.2006 11.5213 14.7006 11.5213 15.3209C11.5213 15.9413 11.0213 16.4413 10.4009 16.4413H9.62126C9.00094 16.4413 8.50094 15.9413 8.50094 15.3209C8.50094 14.7006 9.00094 14.2006 9.62126 14.2006ZM8.50094 10.8404C8.50094 10.2201 9.00094 9.7201 9.62126 9.7201L10.4009 9.72088C11.0213 9.72088 11.5213 10.2209 11.5213 10.8412C11.5213 11.4615 11.0213 11.9615 10.4009 11.9615H9.62126C9.00094 11.9607 8.50094 11.4607 8.50094 10.8404V10.8404ZM14.8407 14.2006H15.6204C16.2407 14.2006 16.7407 14.7006 16.7407 15.3209C16.7407 15.9413 16.2407 16.4413 15.6204 16.4413H14.8407C14.2204 16.4413 13.7204 15.9413 13.7204 15.3209C13.7212 14.7006 14.2212 14.2006 14.8407 14.2006ZM13.7212 10.8404C13.7212 10.2201 14.2212 9.7201 14.8415 9.7201H15.6212C16.2415 9.7201 16.7415 10.2201 16.7415 10.8404C16.7415 11.4607 16.2415 11.9607 15.6212 11.9607H14.8415C14.2212 11.9607 13.7212 11.4607 13.7212 10.8404ZM6.2806 10.8404C6.2806 11.4607 5.7806 11.9607 5.16028 11.9607H4.3806C3.76028 11.9607 3.26028 11.4607 3.26028 10.8404C3.26028 10.2201 3.76028 9.7201 4.3806 9.7201H5.16028C5.7806 9.72088 6.2806 10.2209 6.2806 10.8404ZM4.3806 14.2006H5.16028C5.7806 14.2006 6.2806 14.7006 6.2806 15.3209C6.2806 15.9413 5.7806 16.4413 5.16028 16.4413H4.3806C3.76028 16.4413 3.26028 15.9413 3.26028 15.3209C3.26106 14.7006 3.76106 14.2006 4.3806 14.2006Z" fill="#2683FF"/>
                </svg>
            </div>
        </div>
        <div class="usage-report-container__main-area">
            <div class="usage-report-container__main-area__info-area">
                <div class="usage-report-container__main-area__info-area__item">
                    <h1>Storage, GBh</h1>
                    <h2>{{storage}}</h2>
                </div>
                <div class="usage-report-container__main-area__info-area__item">
                    <h1>Egress, GB</h1>
                    <h2>{{egress}}</h2>
                </div>
                <div class="usage-report-container__main-area__info-area__item">
                    <h1>Object Hours</h1>
                    <h2>{{objectsCount}}</h2>
                </div>
            </div>
            <div class="usage-report-container__main-area__footer">
                <p>Current Roll Up Period <b>{{toLocaleDateString(startDate)}}</b> to <b>{{toLocaleDateString(endDate)}}</b></p>
                <div class="usage-report-container__main-area__footer__report-area">
                    <p>Download Advanced Report</p>
                    <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg" @click.prevent="onReportClick">
                        <rect class="background" width="40" height="40" rx="4"/>
                        <path class="blue" d="M25.6491 19.2809L21.2192 23.5281C20.891 23.8427 20.3988 24 20.0707 24C19.7425 24 19.2503 23.8427 19.0862 23.5281L14.4922 19.2809C13.8359 18.6517 13.8359 17.8652 14.4922 17.236C14.8204 16.9213 15.1485 16.9213 15.6407 16.9213C15.9689 16.9213 16.4611 17.0787 16.6252 17.3933L18.594 19.1236L18.594 11.4157C18.594 10.6292 19.2503 10 20.0707 10C20.891 10 21.5473 10.6292 21.5473 11.4157L21.5473 19.1236L23.5162 17.236C23.6803 16.9213 24.1725 16.9213 24.5006 16.9213C24.8288 16.9213 25.321 17.0787 25.4851 17.3933C26.1414 17.8652 26.1414 18.809 25.6491 19.2809Z"/>
                        <rect class="blue" x="11" y="28" width="18" height="2" rx="1"/>
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
    import { NOTIFICATION_ACTIONS, PROJECT_USAGE_ACTIONS } from '@/utils/constants/actionNames';
    import { toUnixTimestamp } from '@/utils/time';

    @Component({
        components: {
            Datepicker,
        }
    })
    export default class UsageReport extends Vue {
        public startTime: any = {
            time: '',
        };
        private dateRange: any;

        public constructor() {
            super();

            const currentDate = new Date();
            const previousDate = new Date();
            previousDate.setMonth(currentDate.getMonth() - 1);

            this.dateRange = {
                startDate: previousDate,
                endDate: currentDate,
            };
        }

        public get startDate(): Date {
            return this.$store.state.usageModule.startDate;
        }

        public get endDate(): Date {
            return this.$store.state.usageModule.endDate;
        }

        public get storage(): string {
            return this.$store.state.usageModule.projectUsage.storage.toPrecision(5);
        }

        public get egress(): string {
            return this.$store.state.usageModule.projectUsage.egress.toPrecision(5);
        }

        public get objectsCount(): string {
            return this.$store.state.usageModule.projectUsage.objectCount.toPrecision(5);
        }

        public mounted(): void {
            this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP);
        }

        public beforeRouteLeave(to, from, next): void {
            this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP, this.dateRange);

            const buttons = [...(document as any).querySelectorAll('.usage-report-container__options-area__option')];
            buttons.forEach(option => {
                option.classList.remove('active');
            });

            buttons[0].classList.add('active');
            next();
        }

        public onBackClick(): void {
            this.$router.push(ROUTES.PROJECT_OVERVIEW);
        }

        public async onCurrentRollupClick(event: any): Promise<void> {
            this.onButtonClickAction(event);

            const response = await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP);
            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project usage');
            }
        }

        public async onPreviousRollupClick(event: any): Promise<void> {
            this.onButtonClickAction(event);

            const response = await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_PREVIOUS_ROLLUP);
            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project usage');
            }
        }

        public onCustomDateClick(event: any): void {
            (this as any).$refs.datePicker.showCheck();
            this.onButtonClickAction(event);
        }

        public onReportClick(): void {
            const projectID = this.$store.getters.selectedProject.id;
            const startDate = this.$store.state.usageModule.startDate;
            const endDate = this.$store.state.usageModule.endDate;

            let url = new URL(location.origin);

            url.pathname = 'usage-report';
            url.searchParams.append('projectID', projectID);
            url.searchParams.append('since', toUnixTimestamp(startDate).toString());
            url.searchParams.append('before', toUnixTimestamp(endDate).toString());

            window.open(url.href, '_blank');
        }

        public async getDates(datesArray: string[]): Promise<void> {
            const now = new Date();
            const firstDate = new Date(datesArray[0]);
            const secondDate = new Date(datesArray[1]);
            const isInverted = firstDate > secondDate;

            let startDate = isInverted ? secondDate : firstDate;
            let endDate = isInverted ? firstDate : secondDate;

            endDate = new Date(Date.UTC(endDate.getUTCFullYear(), endDate.getUTCMonth(), endDate.getUTCDate(), 23, 59, 59));
            if (now.getUTCFullYear() === endDate.getUTCFullYear() &&
                now.getUTCMonth() === endDate.getUTCMonth() &&
                now.getUTCDate() === endDate.getUTCDate()) {
                endDate = now;
            }

            const response = await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH, {startDate, endDate});
            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project usage');
            }
        }

        public toLocaleDateString(date: Date): string {
            return date.toLocaleDateString('en-US', {timeZone: 'UTC'});
        }

        private onButtonClickAction(event: any): void {
            let eventTarget = event.target;

            if (eventTarget.children.length === 0) {
                eventTarget = eventTarget.parentNode;
            }

            if (eventTarget.classList.contains('active')) {
                return;
            }

            this.changeActiveClass(eventTarget);
        }

        private changeActiveClass(target: any): void {
            this.removeActiveClass();
            target.classList.add('active');
        }

        private removeActiveClass(): void {
            const buttons = [...(document as any).querySelectorAll('.usage-report-container__options-area__option')];
            buttons.forEach(option => {
                option.classList.remove('active');
            });
        }
    }
</script>

<style scoped lang="scss">
    .usage-report-container {
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
            justify-content: flex-start;
            height: 56px;
            
            p {
                font-family: 'font_bold';
                font-size: 24px;
                line-height: 29px;
                color: #354049;
                margin-block-start: 0.5em;
                margin-block-end: 0.5em;
            }
        }
        
        &__options-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: flex-end;
            height: 100%;
            margin-top: 39px;
            
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
                        color: #354049;
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
                    font-size: 16px;
                    line-height: 21px;
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
                        font-size: 16px;
                        line-height: 21px;
                        color: #354049;
                        margin-right: 30px;
                    }
                
                    svg {
                        cursor: pointer;
                        
                        .background {
                            fill: #E2ECF7;
                        }
                        
                        .blue {
                            fill: #2683FF;
                        }
                    }
                
                    svg:hover {
                    
                        .background {
                            fill: #2683FF;
                        }
                        
                        .blue {
                            fill: #FFFFFF;
                        }
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
