// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div v-if="teamArrayLength > 0"  class="team-header">
            <HeaderArea/>
        </div>
        <div v-if="teamArrayLength > 0"  class="team-container">
            <div class="team-container__content">
                <TeamMemberItem class="selected"/>
                <TeamMemberItem/>
            </div>
            <!-- only when selecting team members -->
            <Footer/>
        </div>
        <EmptyState 
            v-if="teamArrayLength === 0"
            mainTitle="Invite Team Members"
            additionalText="You need to click the button “+” in the left corner"
            :imageSource="emptyImage" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import TeamMemberItem from '@/components/team/TeamMemberItem.vue';
import HeaderArea from '@/components/team/headerArea/HeaderArea.vue';
import Footer from '@/components/team/footerArea/Footer.vue';
import EmptyState from '@/components/common/EmptyStateArea.vue';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

@Component({
    data: function() {
            return {
                // TODO: this.$store.state.teamMembers
                teamArrayLength: 1,
                emptyImage: EMPTY_STATE_IMAGES.TEAM
            }
        },
    components: {
        TeamMemberItem,
        HeaderArea,
        Footer,
        EmptyState,
    }
})



export default class TeamArea extends Vue {}
</script>

<style scoped lang="scss">
    .team-header {
        position: fixed;
        padding: 55px 30px 25px 64px;
        max-width: 79.7%;
        width: 100%;
        background-color: rgba(255,255,255,0.6);
        z-index: 999;
    }
    .team-container {
       padding: 0px 30px 55px 64px;
       overflow-y: scroll;
       max-height: 84vh;
       position: relative;

       &__content {
            display: grid;
            grid-template-columns: 260px 260px 260px 260px 260px;
            width: 100%;
            grid-row-gap: 20px;
            justify-content: space-between;
            margin-top: 175px;
            margin-bottom: 100px;
        }
   }

   @media screen and (max-width: 1600px) {
       .team-container {

            &__content {
                grid-template-columns: 240px 240px 240px 240px;
            }
        }
        .team-header {
            max-width: 73.7%;
        }
   }

     @media screen and (max-width: 1366px) {
       .team-container {

            &__content {
                grid-template-columns: 260px 260px 260px;
            }
        }

        .team-header {
            max-width: 72%;
        }
   }

   @media screen and (max-width: 1120px) {
       .team-container {

       &__content {
            grid-template-columns: 270px 270px 270px;
            grid-row-gap: 0px;
            }
        }
        .team-header {
            max-width: 82.7%;
        }
   }
</style>