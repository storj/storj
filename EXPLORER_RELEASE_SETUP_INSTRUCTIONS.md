# The Explorer Release has arrived!

Hello Storj Node Operators! First off, we want to say thank you for your patience. We know many of you have been waiting several months to join the V3 network. Your patience is being rewarded; you are the first nodes to be invited to the Explorer release. This release is gated, which means that we are controlling how many nodes are able to join the network and how quickly they are able to do so. We want to give our early adopters a chance to start earning reputation and STORJ tokens. If we allowed too many nodes to join the network right away, you would be earning fewer STORJ tokens because the available data would naturally be spread over a larger number of nodes. Storj Labs is going to be uploading enough data to the network during this release to ensure all storage nodes get payouts.

####Before you begin
Make sure you have an email with your personal single use authorization token. If you don’t have an authorization token yet, please join our [waitlist](https://storj.io/sign-up-farmer). Install the necessary dependencies and configure your network appropriately using the following steps: 

- Install `docker` please visit: [docker.com](https://docs.docker.com/install/) and follow the installation guide for your operating system. 
- Set up port forwarding! Please visit our [knowledge base article](https://storjlabs.atlassian.net/wiki/spaces/SCKB/pages/edit/4423868?draftId=4292802&draftShareId=dc880538-dc43-4ad1-9691-425adaea7c5c&) or follow the instructions for your router on [portforward.com](https://portforward.com/).

Make sure you have an email with your personal single use authorization token. If you don’t have an authorization token yet, please join our waitlist. You will not be able to setup a storage node if you don't have an authorization token.

#### Setting up your Storage Node on the V3 Network!

1) Download the Identity tool binary and create an Identity. The process of generating an identity could take several hours; it is dependent on your machine´s processing power & luck.

	Download the correct binary for your operating system:
	- Mac OS: [identity_darwin_amd64.zip](https://storj-v3-alpha-builds.storage.googleapis.com/a1027c7-go1.11/identity_darwin_amd64.zip )
	- Linux: [identity_linux_amd64.zip](https://storj-v3-alpha-builds.storage.googleapis.com/a1027c7-go1.11/identity_linux_amd64.zip )
	- Raspberry Pi: [identity_linux_arm.zip](https://storj-v3-alpha-builds.storage.googleapis.com/a1027c7-go1.11/identity_linux_arm.zip )
	- Windows: [identity_windows_amd64.zip](https://storj-v3-alpha-builds.storage.googleapis.com/a1027c7-go1.11/identity_windows_amd64.zip )

2) Unzip the file and run the following command to start creating an identity (this example is for Mac OS, substitute the appropriate identity binary for your OS):

	`$ ./identity_darwin_amd64 create storagenode`

3) Sign the identity you created with your personal single-use authorization token by running the following command: 

	`$ identity authorize storagenode <authorization-token>`

4) Download the docker container from docker hub: 

	`$ docker pull storjlabs/storagenode:alpha`

5) Run storage node with the following command, after editing `WALLET`, `EMAIL`, `ADDRESS`, and `<storage-dir>`
    
	`WALLET`: ethereum address for payments
    `EMAIL`: email address (optional)
    `ADDRESS`: external IP address or the DDNS you configured and the number of the port you manually opened on your router, separated by a colon for example `<ip>:<port>`
    `<storage-dir>`: local directory where you want files to be stored on your hard drive for the network

	`$ docker run -d -e WALLET="" -e EMAIL="" -e ADDRESS="" -v <identity-dir>:/app/identity -v <storage-dir>:/app/config --name storagenode storjlabs/storagenode:alpha`

	*__Caution:__ Before proceeding to the next step, please be sure to back up your identity files located in your ~/identity/storagenode/ folder. This will allow you to restore your node to working order in case of an unfortunate incident such as a hard drive crash.*

6) Start your storage node dashboard by running the following command:

	`$ docker exec -it storagenode dashboard`

7) If step 5 or 6 failed for you, run: 

	`$ docker ps -a`

	Take note of the container ID of the storage node container

	`$ docker logs -t <container-id>`

*If you need help setting up your storage node, sign up for our [community chat](https://community.storj.io/home) and ask for assistance in the #storagenode channel. Provide your logs and stacktrace when requested by the community leader attending your issue.*
