# Using TLS and SSL for Secure node-ipc

### document in progress
Still working on this. If you look at the examples and can help, please jump right in.

#### important cli commands
- openssl genrsa -out server.key 2048
- openssl req -new -x509 -key server.key -out server.pub -days 365 -config openssl.cnf
- openssl req -new -x509 -key client.key -out client.pub -days 365 -config openssl.cnf
- talk about openssl.cnf edits

#### using the local node-ipc certs
This should **ONLY** be done on your local machine. Both the public and private certs are available here on git hub, so its not a good idea to use them over the network.

#### talk about security
- keep private keys private, don't share

#### talk about using hostname not ip for best security validation of certs


#### examples
- basic with default keys
- specikfying keys
- encrypted but venerable to man in the middle
- two way authenticated pub private
