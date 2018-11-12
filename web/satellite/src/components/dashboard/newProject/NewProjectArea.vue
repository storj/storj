<template>
    <div class="npContainer">
        <div class="npButtonContainer" v-on:click="toggleSelection">
            <h1>New Project +</h1>
        </div>
        <NewProjectPopup v-if="isPopupShown" @onClose="toggleSelection"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import NewProjectPopup from "./NewProjectPopup.vue"

// Button and popup for adding new Project
@Component(
    { 
        data: function() {
            return {
                isPopupShown: false
            }
        },
        methods: {
            toggleSelection: function () {
                this.$data.isPopupShown = !this.$data.isPopupShown;
            }
        },
        components: {
            NewProjectPopup
        }
    }
)

export default class NewProjectArea extends Vue {}
</script>

<style scoped lang="scss">
    .npContainer {
        padding-left: 10px;
        padding-right: 10px;
        background-color: #FFFFFF;
    }
    .npButtonContainer {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: center;
        width: 12.5vw;
        height: 5vh;
        border-radius: 6px;
        border: 1px solid #AFB7C1;
        h1 {
            font-family: 'montserrat_medium';
            font-size: 16px;
            line-height: 23px;
            color: #354049;
        }
    }
</style>
