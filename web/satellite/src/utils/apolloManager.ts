import {HttpLink} from "apollo-link-http";
import ApolloClient from "apollo-client/ApolloClient";
import {InMemoryCache} from "apollo-cache-inmemory";

const satelliteUrl = new HttpLink({
    uri: 'http://192.168.1.90:8081/api/graphql/v0',
});

export default  new ApolloClient({
    link: satelliteUrl,
    cache: new InMemoryCache(),
    connectToDevTools: true,
});