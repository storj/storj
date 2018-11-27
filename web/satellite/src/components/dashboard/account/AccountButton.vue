<template>
    <router-link to="/dashboard/account" exact>
        <div class="abContainer">
            <div class="abToggleContainer">
                <!-- background of this div generated and stores in store -->
                <div class="abAvatar" :style="style">
                    <!-- First digit of firstName after Registration -->
                    <!-- img if avatar was set -->
                    <h1>{{avatarLetter}}</h1>
                </div>
                <h1>{{userName}}</h1>
                <div class="abExpanderArea">
                    <img v-if="!isMatchesWithPath" src="../../../../static/images/register/BlueExpand.svg" />
                    <img v-if="isMatchesWithPath" src="../../../../static/images/register/BlueHide.svg" />
                </div>
            </div>
        </div>
    </router-link>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component(
    { 
        data: function() {
            return {
                // check if this.$router.history.current mathes with path 
                isMatchesWithPath: false,
                // this.$store.userName
                userName: "User Name"
            }
        },
        computed: {
            style: function() {
                //color from $store
				return { background: "#95D486" }
            },
            // may change later
            avatarLetter: function() : string {
                return this.$data.userName.slice(0,1).toUpperCase();
            }
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
        padding-left: 10px;
        padding-right: 10px;
        background-color: #FFFFFF;
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