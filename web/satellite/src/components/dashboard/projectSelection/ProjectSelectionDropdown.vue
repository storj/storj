<template>
    <!-- To close popup we need to use method onCloseClick -->
    <div class="psChoiceContainer" >
        <div class="psOverflowContainer">
            <!-- loop for rendering projects -->
            <!-- TODO: add selection logic onclick -->
            <div class="psProjectChoice" v-for="a in this.$data.projects" v-bind:key="a.name" >
                <div class="psMarkContainer">
                    <svg v-if="a.selected" width="15" height="13" viewBox="0 0 15 13" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M14.0928 3.02746C14.6603 2.4239 14.631 1.4746 14.0275 0.907152C13.4239 0.339699 12.4746 0.368972 11.9072 0.972536L14.0928 3.02746ZM4.53846 11L3.44613 12.028C3.72968 12.3293 4.12509 12.5001 4.53884 12.5C4.95258 12.4999 5.34791 12.3289 5.63131 12.0275L4.53846 11ZM3.09234 7.27469C2.52458 6.67141 1.57527 6.64261 0.971991 7.21036C0.36871 7.77812 0.339911 8.72743 0.907664 9.33071L3.09234 7.27469ZM11.9072 0.972536L3.44561 9.97254L5.63131 12.0275L14.0928 3.02746L11.9072 0.972536ZM5.6308 9.97199L3.09234 7.27469L0.907664 9.33071L3.44613 12.028L5.6308 9.97199Z" fill="#2683FF"/>
                    </svg>
                </div>
                <h2 v-bind:class="[a.selected ? 'psSelected' : 'psNotSelected']">{{a.name}}</h2>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component(
    { 
        data: function() {
            return {
                // TODO: format prohect names ( n symbols + ...)
                // Projects is [project]
                // Project here is object with name and selected properties
                projects: [
                    //TODO: change to actual data
                    { name: 'Project name 1', selected: true },
                    { name: 'Project 2 ', selected: false },
                    { name: 'Project 3', selected: false },
                    { name: 'Project 4', selected: false },
                    { name: 'Project 5', selected: false },
                    { name: 'Project 6', selected: false },
                    { name: 'Project 7', selected: false },
                    { name: 'Project 8', selected: false }
                ]
            }
        },
        props: {
            onClose: {
                type: Function
            }
        },
        methods: {
            onCloseClick: function () : void {
                this.$emit("onClose");
            }
        },
    }
)

export default class ProjectSelectionDropdown extends Vue {}
</script>

<style scoped lang="scss">
    .psChoiceContainer {
        position: absolute;
        top: 9vh;
        left: 0px;
        border-radius: 4px;
        padding: 10px 0px 10px 0px;
        box-shadow: 0px 4px rgba(231, 232, 238, 0.6);
        background-color: #FFFFFF;
        z-index: 800;
    }
    .psOverflowContainer {
        position: relative;
        width: 17vw;
        overflow-y: auto;
        overflow-x: hidden;
        height: 25vh;
        background-color: #FFFFFF;
    }
    .psProjectChoice {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        padding-left: 20px;
        padding-right: 20px;
        h2{
            margin-left: 20px; 
            font-size: 14px;
            line-height: 20px;
            color: #354049;
        }
    }
    .psProjectChoice:hover {
        background-color: #F2F2F6;
    }
    .psSelected {
        font-family: 'montserrat_bold';
    }
    .psNotSelected {
        font-family: 'montserrat_regular';
    }
    .psMarkContainer {
        width: 10px;;
        svg {
            object-fit: cover;
        }
    }
    /* width */
    ::-webkit-scrollbar {
        width: 4px;
    }

    /* Track */
    ::-webkit-scrollbar-track {
        box-shadow: inset 0 0 5px #fff; 
    }
    
    /* Handle */
    ::-webkit-scrollbar-thumb {
        background: #AFB7C1; 
        border-radius: 6px;
        height: 5px;
    }
</style>