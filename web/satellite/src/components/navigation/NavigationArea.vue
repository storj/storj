// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="navigation-area">
        <router-link class="navigation-area__item-container" v-for="navItem in navigation" v-bind:key="navItem.label" :to="navItem.path">
            <div class="navigation-area__item-container__link-container" >
                <div v-html="navItem.svg"></div>
                <h1>{{navItem.label}}</h1>
                <div class="navigation-area__item-container__link-container__add-button" id="addTeamMemberPopupButtonSVG" v-if="navItem.label == 'Team'">
                    <div v-on:click="togglePopup">
                        <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <rect width="40" height="40" rx="20" fill="#2683FF"/>
                            <path d="M25 18.977V21.046H20.9722V25H19.0046V21.046H15V18.977H19.0046V15H20.9722V18.977H25Z" fill="white"/>
                        </svg>
                    </div>
                </div>
            </div>
        </router-link>
        <AddUserPopup v-if="isAddTeamMembersPopupShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { mapState } from 'vuex';
import NAVIGATION_ITEMS from '@/utils/constants/navigationLinks';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import AddUserPopup from '@/components/team/AddUserPopup.vue';

@Component(
    {
        data: function () {
            return {
                navigation: NAVIGATION_ITEMS,
                isPopupShown: false,
            };
        },
        components: {
            AddUserPopup,
        },
        methods: {
            togglePopup: function(): void {
                if (!this.$store.getters.selectedProject.id) return;

                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS);
            },
        },
        computed: mapState({
            isAddTeamMembersPopupShown: (state: any) => state.appStateModule.appState.isAddTeamMembersPopupShown,
        }),
    }
)

export default class NavigationArea extends Vue {
}
</script>

<style lang="scss">
    .navigation-area {
        position: relative;
        min-width: 280px;
        max-width: 280px;
        height: 100vh;
        left: 0;
        background-color: #fff;
        padding-top: 3.5vh;

        &__item-container {
             height: 70px;
             padding-left: 60px;
             border-left: 3px solid transparent;
             display: flex;
             justify-content: flex-start;
             align-items: center;
            &.router-link-exact-active,
            &:hover {
                 border-left: 3px solid #2683FF;
                .svg path:not(.white) {
                    fill: #2683FF !important;
                }
            }

            &__link-container {
                 display: flex;
                 flex-direction: row;
                 justify-content: flex-start;
                 align-items: center;
                h1 {
                    font-family: 'montserrat_medium';
                    font-size: 16px;
                    line-height: 23px;
                    color: #354049;
                    margin-left: 15px;;
                }

                &__add-button {
                     margin-left: 40px;
                     background-color: transparent;

                    &:hover {
                        svg {
                            border-radius: 50px;
                            box-shadow: 0px 4px 20px rgba(35, 121, 236, 0.4);
                        }
                    }
                }
            }
        }
    }

    a {
        text-decoration: none;
        outline: none;
    }

    @media screen and (max-width: 1024px) {
        .navigation-area {
            width: 80px;
            max-width: 80px;
            min-width: 80px;

            &__item-container {
                 padding-left: 26px;

                &__link-container {
                    h1 {
                        display: none;
                    }

                    &__add-button {
                         display: none;
                    }
                }
            }
        }
    }
</style>
