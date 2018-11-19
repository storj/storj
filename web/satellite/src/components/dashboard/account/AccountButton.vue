<template>
    <div class="abContainer" >
        <div class="abToggleContainer" v-on:click="toggleSelection" >
            <!-- background of this div generated and stores in store -->
            <div class="abAvatar" :style="style">
                <!-- First digit of firstName after Registration -->
                <!-- img if avatar was set -->
                <h1>{{avatarLetter}}</h1>
            </div>
            <h1>{{userName}}</h1>
            <div class="abExpanderArea">
                <img v-if="!isChoiceShown" src="../../../../static/images/register/BlueExpand.svg" />
                <img v-if="isChoiceShown" src="../../../../static/images/register/BlueHide.svg" />
            </div>
        </div>
        <AccountDropdown v-if="isChoiceShown" @onClose="toggleSelection" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import AccountDropdown from "./AccountDropdown.vue";

@Component(
    { 
        data: function() {
            return {
                // this.$store.userName
                userName: "User Name",
                isChoiceShown: false
            }
        },
        computed: {
            style: function() : object {
                //color from $store
				return { background: "#95D486" }
            },
            // may change later
            avatarLetter: function() : string {
                return this.$data.userName.slice(0,1).toUpperCase();
            }
        },
        methods: {
            toggleSelection: function() : void {
                this.$data.isChoiceShown = !this.$data.isChoiceShown;
            }
        },
        components: {
            AccountDropdown
        }
    }
)

export default class AccountButton extends Vue {}
</script>

<style scoped lang="scss">
    a {
        text-decoration: none;
        outline: none;
    }
    .abContainer {
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
    .abToggleContainer {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: space-between;
        width: 12.5vw;
        height: 5vh;
    }
    .abAvatar {
        width: 2.8vw;
        height: 100%;
        border-radius: 6px;
        display: flex;
        align-items: center;
        justify-content: center;
        h1 {
            font-family: 'montserrat_medium';
		    font-size: 16px;
		    line-height: 23px;
            color: #354049;
        }
    }
    .abExpanderArea {
        display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
    }
</style>