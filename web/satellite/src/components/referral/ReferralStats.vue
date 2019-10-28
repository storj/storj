// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="referral-stats">
        <p class="referral-stats__title"> {{ title }} </p>
        <div class="referral-stats__wrapper">
            <div class="referral-stats__card"
                v-for="(stat, key) in stats"
                :key="key"
                :style="stat.background">
                <span class="referral-stats__card-text">
                    <span class="referral-stats__card-title">{{ stat.title }}</span>
                    <span class="referral-stats__card-description">{{ stat.description }}</span>
                </span>
                <br>
                <span class="referral-stats__card-number">{{ stat.symbol + usage[key] }}</span>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { CREDIT_USAGE_ACTIONS } from '@/store/modules/credits';
import { CreditUsage } from '@/types/credits';

class CreditDescription {
    public title: string;
    public description: string;
    public symbol: string;

    // possibly we could add some 'style' type
    public style: any;

    constructor(title: string, description: string, symbol: string, color: string) {
        this.title = title;
        this.description = description;
        this.symbol = symbol;
        this.style = { backgroundColor: color };
    }
}

@Component
export default class ReferralStats extends Vue {
    private readonly TITLE_SUFFIX: string = 'Here Are Your Referrals So Far';
    private stats: CreditDescription[] = [
        new CreditDescription('referrals made', 'People you referred who signed up', '', '#FFFFFF'),
        new CreditDescription('earned credits', 'Free credits that will apply to your upcoming bill', '$', 'rgba(217, 225, 236, 0.5)'),
        new CreditDescription('applied credits', 'Free credits that have already been applied to your bill', '$', '#D1D7E0'),
    ];

    public async mounted() {
        // TODO: we pre-fetch all data in /src/views/Dashboard, but this is tardigrade related, so could be here
        await this.fetch();
    }

    public async fetch(): Promise<void> {
        try {
            await this.$store.dispatch(CREDIT_USAGE_ACTIONS.FETCH);
        } catch (error) {
            await this.$notify.error('Unable to fetch credit usage: ' + error.message);
        }
    }

    public get title(): string {
        // TODO: not sure that we are able to create a user with empty name
        const name = this.$store.state.usersModule.fullName || '' ;

        return name.length > 0 ? `${name}, ${this.TITLE_SUFFIX}` : this.TITLE_SUFFIX;
    }

    public get usage(): CreditUsage {
        return this.$store.getters.credits;
    }
}
</script>

<style scoped lang="scss">
    .referral-stats {

        &__title {
            text-align: center;
            font-family: 'font_bold', sans-serif;
        }

        &__wrapper {
            display: flex;
            flex-direction: row;
            justify-content: space-around;
            left: 15%;
            right: 15%;
            font-family: 'font_regular', sans-serif;
        }

        &__card {
            color: #354049;
            min-height: 176px;
            max-width: 276px;
            flex-basis: 25%;
            border-radius: 24px;
            padding-top: 25px;
            padding-left: 26px;
            padding-right: 29px;
            display: flex;
            flex-direction: column;
            justify-content: space-evenly;

            &-text {
                display: block;
            }

            &-title {
                display: block;
                text-transform: uppercase;
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                line-height: 18px;
            }

            &-description {
                display: block;
                font-size: 14px;
                line-height: 21px;
                margin-top: 7px;
            }

            &-number {
                display: block;
                font-family: 'font_bold', sans-serif;
                font-size: 46px;
                line-height: 60px;
                margin-bottom: 27px;
            }
        }
    }
</style>
