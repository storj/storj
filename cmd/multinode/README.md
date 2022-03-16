# Multinode Dashboard

## Reference Articles

[Tech Preview Forum Post](https://forum.storj.io/t/tech-preview-multinode-dashboard-binaries/14572)

## Generate identity files

In order to run this Docker image, you need to create an identity for the Multinode Dashboard.
For this you need the binaries of the Identity Tool, which you can find in the latest version here:

[https://github.com/storj/storj/releases/latest](https://github.com/storj/storj/releases/latest)

In this example under Windows, we use the file `identity_windows_amd64.zip`, download it and unzip it.
Then we open a PowerShell window in the folder where the `identity.exe` was unzipped and run the following command:
```
./identity.exe create multinode --difficulty 10
```

If we run this command on Windows, the identity files will be created in the folder `%appdata%\Storj\Identity\multinode`.

## Running the Multinode Dashboard in Docker

Then start the image like this, while replacing the directories marked by the `< >` with your parameters below:

```
docker run -d --restart unless-stopped \
    --user $(id -u):$(id -g) \
    -p 127.0.0.1:15002:15002/tcp \
    --mount type=bind,source="<multinode-identity-dir>",destination=/app/identity \
    --mount type=bind,source="<multinode-config-dir>",destination=/app/config \
    --name multinode storjlabs/multinode:latest
```
