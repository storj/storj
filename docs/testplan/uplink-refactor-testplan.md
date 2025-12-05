# Uplink Refactor Testplan

&nbsp;

## Background

This testplan is going to cover the new Uplink. The goal is to test its performance.

&nbsp;

&nbsp;

| Test Scenario            | Test Cases                   | Description                                                                                                                                                                                           | Comments                                                                                             |
|--------------------------|------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------|
| Performance              |  Small file                | Do the upload for 1 KiB, 5 KiB, 1 MiB, 64 MiB files. We want to compare performance with the old uplink implementation. Test the memory consumption for uploads                                       | https://thanos.storj.rodeo/d/5bCCsabVk/uplink-upload-refactor-comparison?orgId=1&from=now-24h&to=now |
|                          |  Big file                  | Do the upload 1024Mb files. We want to compare performance with the old uplink implementation. Test single file upload and several in parallel                                                        |                                                                                                      |
| Uplink cli               |  ls operations             | List buckets, list objects performance with more than 1000 objects                                                                                                                                    |                                                                                                      |
|                          |  Multipart uploads         | Customers should get the performance benefits of Uplink cli. So we can test it by uploading a 1024Mb file and it shouldn't be a multipart upload                                                      |                                                                                                      |
| 3d party tools           |  Filebrowser               | We should be able to keep the same level of operations with FileZilla, all functional of uplink should work. Performance improvements should be available there                                       |                                                                                                      |
|                          |  Rclone                    | We should be able to keep the same level of operations with rclone, all functions of uplink should work. Performance improvements should be available there                                           |                                                                                                      |  
| Low bandwidth situations |  Upload failed(<80 pieces) | Limit the bandwidth of uplink cli for the upload. The old uplink failed but the new implementation should succeed                                                                                     |                                                                                                      |
| Retry limit              |  Uplink behind a firewall  | Close all ports except the port for satellite communication. Expected result: we don't want an endless loop, a user should have some error message after several attempts                             |                                                                                                      |
| Commit upload endpoint   |  Signed piece hash        | We need to check if we have an automated test. To test that uplink unable to commit segment without providing a signed piece hash. And the new uplink still needs to call the CommitSegment function. |                                                                                                      |
|                          |  Duplicate pieces         | Request 80 pieces and claim that they didn't work, request additional 80 pieces and try to commit all 160 pieces as a result there shouldn't be duplicates in it                                      |                                                                                                      |