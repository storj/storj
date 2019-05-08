package io.storj.mobile.libuplink;

import android.support.test.runner.AndroidJUnit4;
import android.support.test.InstrumentationRegistry;

import org.junit.Assert;
import org.junit.Test;
import org.junit.runner.RunWith;

import io.storj.libuplink.mobile.BucketInfo;
import io.storj.libuplink.mobile.BucketList;
import io.storj.libuplink.mobile.Config;
import io.storj.libuplink.mobile.Project;
import io.storj.libuplink.mobile.ProjectOptions;
import io.storj.libuplink.mobile.Uplink;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.fail;

@RunWith(AndroidJUnit4.class)
public class LibuplinkInstrumentedTest {

    public static final String VALID_SATELLITE_ADDRESS = "10.0.2.2:10000";
    public static final String VALID_API_KEY = InstrumentationRegistry.getArguments().getString("api.key");

    @Test
    public void testOpenProjectFail() throws Exception {
        Config config = new Config();
        config.setIdentity(null);

        Uplink uplink = new Uplink(config);
        try {
            ProjectOptions options = new ProjectOptions();
            options.setEncryptionKey("TestEncryptionKey".getBytes());

            Project project = null;
            try {
                // 10.0.2.2 refers to not existing satellite
                project = uplink.openProject("10.0.2.2:1", VALID_API_KEY, options);
                fail("exception expected");
            } catch (Exception e) {
                // skip
            } finally {
                if (project != null) {
                    project.close();
                }
            }
        } finally {
            uplink.close();
        }
    }

    @Test
    public void testBasicBucket() throws Exception {
        Config config = new Config();
        config.setIdentity(null);

        Uplink uplink = new Uplink(config);
        try {
            ProjectOptions options = new ProjectOptions();
            options.setEncryptionKey("TestEncryptionKey".getBytes());

            Project project = null;
            try {
                project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY, options);

                String expectedBucket = "testBucket";
                project.createBucket(expectedBucket);
                BucketInfo bucketInfo = project.getBucketInfo(expectedBucket);
                Assert.assertEquals(expectedBucket, bucketInfo.getName());

                project.deleteBucket(expectedBucket);

                try {
                    project.getBucketInfo(expectedBucket);
                } catch (Exception e) {
                    Assert.assertTrue(e.getMessage().startsWith("bucket not found"));
                }
            } finally {
                if (project != null) {
                    project.close();
                }
            }
        } finally {
            uplink.close();
        }
    }

    @Test
    public void testListBuckets() throws Exception {
        Config config = new Config();
        config.setIdentity(null);

        Uplink uplink = new Uplink(config);
        try {
            ProjectOptions options = new ProjectOptions();
            options.setEncryptionKey("TestEncryptionKey".getBytes());

            Project project = null;
            try {
                project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY, options);

                for (int i = 0; i < 10; i++) {
                    String expectedBucket = "testBucket" + i;
                    project.createBucket(expectedBucket);
                }

                BucketList items = project.listBuckets("", 1, 100);
                assertEquals(false, items.more());
                assertEquals(10, items.length());

                for (int i = 0; i < 10; i++) {
                    String expectedBucket = "testBucket" + i;
                    project.deleteBucket(expectedBucket);
                }
            } finally {
                if (project != null) {
                    project.close();
                }
            }
        } finally {
            uplink.close();
        }
    }
}