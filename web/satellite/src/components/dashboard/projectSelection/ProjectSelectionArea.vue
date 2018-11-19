<template>
    <div class="psContainer">
        <div class="psToggleContainer" v-on:click="toggleSelection">
            <h1>{{projectName}}</h1>
            <div class="psExpanderArea">
                <img v-if="!isChoiceShown" src="../../../../static/images/register/BlueExpand.svg" />
                <img v-if="isChoiceShown" src="../../../../static/images/register/BlueHide.svg" />
            </div>
        </div>
        <ProjectSelectionDropdown v-if="isChoiceShown" @onClose="toggleSelection"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import ProjectSelectionDropdown from "./ProjectSelectionDropdown.vue"

@Component(
    { 
        data: function() {
            return {
                isChoiceShown: false,
                // this.$store.selectedProject
                projectName: "Choose Project"
            }
        },
        methods: {
            toggleSelection: function () {
                this.$data.isChoiceShown = !this.$data.isChoiceShown;
            }
        },
        components: {
            ProjectSelectionDropdown
        }
    }
)

export default class ProjectSelectionArea extends Vue {}
</script>

<style scoped lang="scss">
    .psContainer {
        position: relative;
        padding-left: 10px;
        padding-right: 10px;
        background-color: #FFFFFF;
        cursor: pointer;
        h1 {
            font-family: 'montserrat_medium';
            font-size: 16px;
            line-height: 23px;
            color: #354049;
        }
    }
    .psToggleContainer {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: space-between;
        width: 12.5vw;
        height: 5vh;
    }
    .psExpanderArea {
        display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
    }
</style>