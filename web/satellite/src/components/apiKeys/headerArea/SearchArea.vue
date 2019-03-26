// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="search-container">
        <div class="search-container__wrap">
            <label class="search-container__wrap__input">
                <div v-if="searchQuery" class="search-container__wrap__input__clear" v-on:click="clearSearch" >
                    <svg width="12" height="12" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                    </svg>
                </div>
                <input v-model="searchQuery" placeholder="Search API" type="text">
            </label>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component({
    data:function () {
        return {
            searchQuery:''
        };
    },
    methods: {
        clearSearch: function () {
            this.$data.searchQuery = '';
        }
    }
})

export default class SearchArea extends Vue {
}
</script>

<style scoped lang="scss">
    .search-container {
        width: 100%;
        height: 56px;
        margin-right: 24px;

        &__wrap {
            position: relative;

            &::after {
                content: '';
                display: block;
                position: absolute;
                height: 20px;
                top: 50%;
                transform: translateY(-50%);
                right: 20px;
                width: 20px;
                background-image: url('../../../../static/images/team/searchIcon.svg');
                background-repeat: no-repeat;
                background-size: cover;
                z-index: 20;
            }

            &__input {

                &__clear {
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    position: absolute;
                    right: 65px;
                    top: 50%;
                    transform: translateY(-50%);
                    z-index: 99;
                    cursor: pointer;
                    height: 20px;
                    width: 20px;

                    &:hover svg path {
                        fill: #2683FF;
                    }
                }

                input {
                    box-sizing: border-box;
                    position: relative;
                    border: none;
                    outline: none;
                    border-radius: 6px;
                    width: 100%;
                    height: 56px;
                    padding-right: 20px;
                    padding-left: 20px;
                    font-family: 'font_regular';
                    font-size: 16px;
                    transition: all .2s ease-in-out;

                    &:hover {
                        box-shadow: 0px 4px 4px rgba(231, 232, 238, 0.6);
                        border: none;
                        outline: none;
                    }

                    &:focus {
                        box-shadow: 0px 4px 4px rgba(231, 232, 238, 0.6);
                        border: none;
                        outline: none;
                    }
                }
            }
        }
    }

    ::-webkit-input-placeholder {
        color: #AFB7C1;
    }
</style>