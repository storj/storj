// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="team-header">
            <HeaderArea/>
        </div>
        <div id="scrollable_team_container" v-if="projectMembers.length > 0" v-on:scroll="handleScroll" class="team-container">
            <div class="team-container__content">
                <div v-for="(member, index) in projectMembers" v-on:click="onMemberClick(member)" v-bind:key="index">
                    <TeamMemberItem
                        :projectMember = "member"
                        v-bind:class = "[member.isSelected ? 'selected' : '']"
                    />
                </div>
            </div>
            <!-- only when selecting team members -->
            <div v-if="selectedProjectMembers.length > 0" >
                <Footer/>
            </div>
        </div>
        <EmptyState
                v-if="projectMembers.length === 0"
                mainTitle="No results found"
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
import { NOTIFICATION_ACTIONS, PM_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    data: function () {
        return {
            emptyImage: EMPTY_STATE_IMAGES.TEAM,
            isFetchInProgress: false,
        };
    },
    methods: {
        onMemberClick: function (member: any) {
            this.$store.dispatch(PM_ACTIONS.TOGGLE_SELECTION, member.user.id);
		},
		handleScroll: async function () {
			const documentElement = document.getElementById('scrollable_team_container');
			if (!documentElement) {
				return;
			}

			const isAtBottom = documentElement.scrollTop + documentElement.clientHeight === documentElement.scrollHeight;

			if (!isAtBottom || this.$data.isFetchInProgress) return;

			this.$data.isFetchInProgress = true;

			const response = await this.$store.dispatch(PM_ACTIONS.FETCH);

			this.$data.isFetchInProgress = false;

			if (response.isSuccess) return;

			this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project members');
        },
    },
    computed: {
        projectMembers: function () {
            return this.$store.getters.projectMembers;
        },
        selectedProjectMembers: function () {
            return this.$store.getters.selectedProjectMembers;
        },
    },
    components: {
        TeamMemberItem,
        HeaderArea,
        Footer,
        EmptyState,
    }
})

export default class TeamArea extends Vue {
}
</script>

<style scoped lang="scss">
    .team-header {
        position: fixed;
        top: 100px;
        padding: 55px 30px 0px 64px;
        max-width: 79.7%;
        width: 100%;
        background-color: #F5F6FA;
        z-index: 999;
    }
    .team-container {
       padding: 0px 30px 55px 64px;
       overflow-y: scroll;
       max-height: 84vh;
       position: relative;

       &__content {
            display: grid;
            grid-template-columns: 230px 230px 230px 230px 230px 230px;
            width: 100%;
            grid-column-gap: 20px;
            grid-row-gap: 20px;
            justify-content: space-between;
            margin-top: 150px;
            margin-bottom: 100px;
        }
   }

    .user-container {
        height: 160px;
    }

   @media screen and (max-width: 1600px) {
       .team-container {

            &__content {
                grid-template-columns: 220px 220px 220px 220px 220px;
            }
       }

        .team-header {
            max-width: 75%;
        }

       .user-container {
           height: 160px;
       }
   }

    @media screen and (max-width: 1366px) {
       .team-container {

            &__content {
                grid-template-columns: 210px 210px 210px 210px;
            }
        }

        .team-header {
            max-width: 70.2%;
        }

         .user-container {
             height: 160px;
         }
   }

   @media screen and (max-width: 1120px) {
       .team-container {

           &__content {
                grid-template-columns: 200px 200px 200px 200px;
            }
        }
        .team-header {
            max-width: 82.7%;
        }

       .user-container {
           height: 150px;
       }
   }
</style>