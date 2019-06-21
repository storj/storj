<template>
    <div class="referral-stats">
        <div class="referral-stats__wrapper"
            v-for="(stat, key) in stats"
            :key="key"
            :style="stat.background">
            <span class="referral-stats__text">
                <span class="referral-stats__title">{{ stat.title }}</span>
                <span class="referral-stats__description">{{ stat.description }}</span>
            </span>
            <br>
            <span class="referral-stats__number">{{ usage[key] }}</span>
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
                 background: {
                     backgroundColor: "#FFFFFF",
                 }
             },
              availableCredits: {
                 title: "earned credits",
                 description: "Free credits that will apply to your upcoming bill",
               background: {
                     backgroundColor: "rgba(217, 225, 236, 0.5)",
                 }
              },
              usedCredits: {
                 title: "applied credits",
                 description: "Free credits that have already been applied to your bill",
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
        this.fetch();
    },
    computed: {
        usage() {
            return this.$store.state.creditUsageModule.creditUsage;
        }
    }
})

export default class ReferralStats extends Vue {}
</script>

<style scoped lang="scss">
.referral-stats {
    display: flex;
    flex-direction: row;
    justify-content: space-around;
    left: 15%;
    right: 15%;

    &__wrapper {
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
     }
        &__text {
            display: block;
         }
     &__title {
        display: block;
        text-transform: uppercase;
        font-family: 'Roboto';
        font-style: normal;
        font-weight: 500;
        font-size: 14px;
        line-height: 18px;
      }

      &__description {
        display: block;
        font-family: Roboto;
        font-style: normal;
        font-weight: normal;
        font-size: 14px;
        line-height: 21px;
        margin-top: 7px;
       }

       &__number {
            display: block;
            font-family: Roboto;
            font-style: normal;
            font-weight: bold;
            font-size: 46px;
            line-height: 60px;
            margin-bottom: 27px;
        }
}
</style>