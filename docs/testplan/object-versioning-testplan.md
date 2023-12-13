# Object Versioning

## Background

This testplan covers Object Versioning
&nbsp;

| Test Scenario | Test Case                               | Description                                                                                                                                                                                     | Comments                                                  |   
|---------------|-----------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------|
| Copy          | To a bucket that has versioning enabled | Should add one version to it(make a new version and make it latest)                                                                                                                             | check the column "versioning_state" in  "bucket_metainfo" |  
|               | To a bucket that has version disabled   | Regular copy                                                                                                                                                                                    |                                                           |
|               | Copy object version                     | Should support copying a specific version, should copy the latest version of an object if not specified                                                                                         |                                                           |
| Move          | To a bucket that has versioning enabled | Should add one version to it                                                                                                                                                                    | check the column "versioning_state" in  "bucket_metainfo" | 
|               | To a bucket that has version disabled   | Regular move                                                                                                                                                                                    |                                                           |
| Delete        | Delete one from many versions           | Create 3 versions of the same file and delete the middle one indicating the version id                                                                                                          |                                                           |
|               | All versions                            | Unconditionally deletes all versions of an object                                                                                                                                               |                                                           |
|               | Delete bucket                           | Force delete bucket with files that has versioning. We should keep all versions of the files unless manually deleted                                                                            |                                                           |
|               | Delete marker                           | Create delete marker by delete specific version and then delete the delete marker. All versions of the file should kept, delete marker should be deleted                                        |                                                           |
| Restore       | Delete and restore                      | Delete version of the file and restore from that version                                                                                                                                        |                                                           |
|               | Restore                                 | Create few versions of the file and restore from latest to older version                                                                                                                        |                                                           |
| Create        | Create new bucket                       | Versioning should be inherited from project level                                                                                                                                               |                                                           |
| Suspend       | Suspend versioning                      | Suspend versioning on a bucket that had versioning enabled. 3 versions of a file exists. Try to upload the same file again. -> the newest file gets overriden. The older 2 versions stay intact |                                                           |
| Update        | Update metadata                         | Metadata update should not create new version. Takes the version as input but does not use it. Only updates the metadata for the highest committed object version.                              |                                                           |
| List          | all versions                            | Unconditionally returns all object versions. Listing all versions should include delete markers. Versions come out created last to first                                                        |                                                           |
| UI            | UI                                      | UI should always show the latest version of each object                                                                                                                                         |                                                           |
| Buckets       | Old                                     | Old buckets created before the feature should be in "unsupported" state                                                                                                                         |                                                           |
|               | Enable versioning after upload          | Upload obj to a bucket with versioning disabled and then enable versioning. Check version of the object                                                                                         |                                                           |
| PutObject     | Versioning enabled                      | When object with same name uploaded to a bucket we should create new unique version of the object                                                                                               |                                                           |
|               | Versioning disabled                     | Latest version of the object is overwritten by the new object, new object has a version ID of null                                                                                              |                                                           |
|               | Multipart                               | Multipart upload with versioning enabled                                                                                                                                                        |                                                           |
|               | Expiration                              | Create object with expiration in versioned bucket, delete marker should be applied to it                                                                                                        |                                                           |

## Third-party test suite

These test suites have good tests inside, so we should run all versioning
related tests in them

* https://github.com/ceph/s3-tests/blob/master/s3tests_boto3/functional/test_s3.py
* https://github.com/snowflakedb/snowflake-s3compat-api-test-suite

## Questions

* Can a customer set a maximum number of versions?
* Can a customer pin specific versions to make sure they can't be deleted
  by malware?
* Can a project member with a restricted access grant modify the version
  flag on a bucket? Which permissions does the access grant need?