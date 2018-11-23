import Vue from 'vue';
import App from './App.vue';
import router from './router';
import store from './store';
import {HttpLink} from "apollo-link-http";
import ApolloClient from "apollo-client/ApolloClient";
import {InMemoryCache} from "apollo-cache-inmemory";
import VueApollo from "vue-apollo";

Vue.config.productionTip = false;

const httpLink = new HttpLink({
    uri: 'http://192.168.1.90:8081/api/graphql/v0',
});

const apolloClient = new ApolloClient({
    link: httpLink,
    cache: new InMemoryCache(),
    connectToDevTools: true,
});

const apolloProvider = new VueApollo({
    defaultClient: apolloClient,

});

Vue.use(VueApollo)

new Vue({
    router,
    store,
    apolloProvider,
    render: (h) => h(App),
}).$mount('#app');
