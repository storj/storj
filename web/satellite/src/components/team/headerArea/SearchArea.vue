// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="search-container">
        <div class="search-container__wrap">
            <label class="search-container__wrap__input">
                <input v-on:input="processSearchQuery" v-model="searchQuery" placeholder="Search Users" type="text">
            </label>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { NOTIFICATION_ACTIONS, PM_ACTIONS } from '@/utils/constants/actionNames';

@Component({
	data:function () {
		return {
			searchQuery:''
		};
	},
	methods: {
		processSearchQuery: async function () {
			this.$store.dispatch(PM_ACTIONS.SET_PROJECT_MEMBERS_SEARCH_QUERY, this.$data.searchQuery);
			const response = await this.$store.dispatch(PM_ACTIONS.FETCH);

			if (response.isSuccess) return;

			this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project members');
		},
	}
})

export default class SearchArea extends Vue {
}
</script>

<style scoped lang="scss">
    .search-container {
        width: 100%;
        height: 56px;
        margin: 0 24px;

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
                    font-family: 'montserrat_regular';
                    font-size: 16px;
                    color: #AFB7C1;
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