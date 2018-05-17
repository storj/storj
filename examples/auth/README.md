# Intro: Auth Package

The auth package is integrated to the server side code in /storj/netstate/routes. This example is client side code of how to make a request with the X-Api-Key header, where the server will validate the proper API credentials and proceed or err on the request. 

## How to Use the Auth Package
1) Run the server:
    ``` go run storj/cmd/netstate-http/main.go```
2) Run the client:
    ```go run storj/examples/auth/main.go```

    you should get a HTTP 201 back for the PUT request. If you change the client-side api creds to a different value, you will get 401 unauthorized error. 

3) You can also test this with POSTMAN with creds listed on the client-side code. 