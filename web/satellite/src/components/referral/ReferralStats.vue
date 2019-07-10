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
    import { CREDIT_USAGE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
        data() {
            return {
                stats: {
                    referred: {
                        title: "referrals made",
                        description: "People you referred who signed up",
                        symbol: "",
                        background: {
                            backgroundColor: "#FFFFFF",
                        }
                    },
                    availableCredits: {
                        title: "earned credits",
                        description: "Free credits that will apply to your upcoming bill",
                        symbol: "$",
                        background: {
                            backgroundColor: "rgba(217, 225, 236, 0.5)",
                        }
                    },
                    usedCredits: {
                        title: "applied credits",
                        description: "Free credits that have already been applied to your bill",
                        symbol: "$",
                        background: {
                            backgroundColor: "#D1D7E0",
                        }
                    },
                }
            }
        },
        methods: {
            fetch: async function() {
                const creditUsageResponse = await this.$store.dispatch(CREDIT_USAGE_ACTIONS.FETCH);
                if (!creditUsageResponse.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch credit usage: ' + creditUsageResponse.errorMessage);
                }
            }
        },
        mounted() {
            (this as any).fetch();
        },
        computed: {
            title() {
                let name = this.$store.state.usersModule.fullName || "" ;
                let text = "Here Are Your Referrals So Far";
                return name.length > 0 ? `${name}, ${text}` : text;
            },
            usage() {
                return this.$store.state.creditUsageModule.creditUsage;
            }
        }
    })

    export default class ReferralStats extends Vue {}
</script>

<style scoped lang="scss">
    .referral-stats {

        &__title {
            text-align: center;
            font-family: 'font_bold';
        }

        &__wrapper {
            display: flex;
            flex-direction: row;
            justify-content: space-around;
            left: 15%;
            right: 15%;
            font-family: 'font_regular';
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
                font-family: 'font_bold';
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
                font-family: 'font_bold';
                font-size: 46px;
                line-height: 60px;
                margin-bottom: 27px;
            }
        }
    }
</style>
