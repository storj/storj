# Multinode Dashboard

## Reference Articles

[Tech Preview Forum Post](https://forum.storj.io/t/tech-preview-multinode-dashboard-binaries/14572)

## Running in Docker

In order to run this docker image, you need to generate an identity like this:
```
identity create multinode --difficulty 10
```

Then start the image like this, while replacing the directories marked by the `< >` with your parameters below:

```
docker run -d --restart unless-stopped \
    --user $(id -u):$(id -g) \
    -p 127.0.0.1:15002:15002/tcp \
    --mount type=bind,source="<identity-dir>",destination=/app/identity \
    --mount type=bind,source="<storage-dir>",destination=/app/config \
    --name multinode storjlabs/multinode:latest
```
