<template>
    <div class="navigation-area">
        <div class="navigation-area__item-container" v-for="navItem in navigation" v-bind:key="navItem.label" >
            <router-link class="navigation-area__item-container__link-container" :to="navItem.path">
                <div v-html="navItem.svg"></div>
                <h1>{{navItem.label}}</h1>
                <div class="navigation-area__item-container__link-container__add-button" v-if="navItem.label == 'Team'">
                    <router-link to="/team/add_new">
                        <img src="../../../static/images/dashboard/Add.png" />
                    </router-link>
                </div>
            </router-link>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import NAVIGATION_ITEMS from "@/utils/constants/navigationLinks";

@Component(
    { 
        data: function() {
            return {
                navigation: NAVIGATION_ITEMS
            }
        }
    }
)

export default class NavigationArea extends Vue {}
</script>

<style lang="scss">
    .navigation-area {
        position: fixed;
        width: 20vw;
		height: 100%;
		left: 0;
		top: 10vh;
        background-color: #fff;
        padding-top: 3.5vh;

        &__item-container {
            height: 7.5vh;
            padding-left: 4vw;
            border-left: 3px solid transparent;
            display: flex;
            justify-content: flex-start;
            align-items: center;

            &:hover {
                border-left: 3px solid #2683FF;
                svg path {
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
                    margin-left: 4vw;
                    background-color: transparent;

                    &:hover {
                         img {
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
    }
</style>
