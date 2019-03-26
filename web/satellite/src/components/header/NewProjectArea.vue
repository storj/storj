// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="new-project-container">
        <div class="new-project-button-container" v-on:click="toggleSelection" id="newProjectButton">
            <h1>New Project +</h1>
        </div>
        <NewProjectPopup v-if="isPopupShown"/>
    </div>
</template>

<script lang="ts">
import { mapState } from 'vuex';
import { Component, Vue } from 'vue-property-decorator';
import NewProjectPopup from '@/components/project/NewProjectPopup.vue';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

// Button and popup for adding new Project
@Component(
    {
        methods: {
            toggleSelection: function () {
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);
            }
        },
        components: {
            NewProjectPopup
        },
        computed: mapState({
            isPopupShown: (state: any) => state.appStateModule.appState.isNewProjectPopupShown,
        }),
    }
)

export default class NewProjectArea extends Vue {
}
</script>

<style scoped lang="scss">
    .new-project-container {
        padding-left: 10px;
        padding-right: 10px;
        background-color: #FFFFFF;
        cursor: pointer;
    }

    .new-project-button-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: center;
        width: 170px;
        height: 50px;
        border-radius: 6px;
        border: 1px solid #AFB7C1;
        background-color: white;


        h1 {
            font-family: 'font_medium';
            font-size: 16px;
            line-height: 23px;
            color: #354049;
        }

        &:hover {
            background-color: #2683FF;
            border: 1px solid #2683FF;
            box-shadow: 0px 4px 20px rgba(35, 121, 236, 0.4);

            h1 {
                color: white;
            }
        }
    }
</style>
