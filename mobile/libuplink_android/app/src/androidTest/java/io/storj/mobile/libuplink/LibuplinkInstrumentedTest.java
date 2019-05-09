package io.storj.mobile.libuplink;

import android.support.test.runner.AndroidJUnit4;
import android.support.test.InstrumentationRegistry;

import org.junit.Assert;
import org.junit.Test;
import org.junit.runner.RunWith;

import java.io.BufferedOutputStream;
import java.io.BufferedWriter;
import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.util.HashSet;
import java.util.Random;
import java.util.Set;

import io.storj.libuplink.mobile.Bucket;
import io.storj.libuplink.mobile.BucketAccess;
import io.storj.libuplink.mobile.BucketConfig;
import io.storj.libuplink.mobile.BucketInfo;
import io.storj.libuplink.mobile.BucketList;
import io.storj.libuplink.mobile.Config;
import io.storj.libuplink.mobile.Project;
import io.storj.libuplink.mobile.ProjectOptions;
import io.storj.libuplink.mobile.Reader;
import io.storj.libuplink.mobile.ReaderOptions;
import io.storj.libuplink.mobile.RedundancyScheme;
import io.storj.libuplink.mobile.Uplink;
import io.storj.libuplink.mobile.Writer;
import io.storj.libuplink.mobile.WriterOptions;

import static org.junit.Assert.assertArrayEquals;
import static org.junit.Assert.assertEquals;
import static org.junit.Assert.fail;

@RunWith(AndroidJUnit4.class)
public class LibuplinkInstrumentedTest {

    public static final String VALID_SATELLITE_ADDRESS = "192.168.8.134:10000";
    public static final String VALID_API_KEY = InstrumentationRegistry.getArguments().getString("api.key");

    @Test
    public void testOpenProjectFail() throws Exception {
        Config config = new Config();

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

        Uplink uplink = new Uplink(config);
        try {
            ProjectOptions options = new ProjectOptions();
            options.setEncryptionKey("TestEncryptionKey".getBytes());

            Project project = null;
            try {
                project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY, options);

                String expectedBucket = "testBucket";
                project.createBucket(expectedBucket, new BucketConfig());
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

        Uplink uplink = new Uplink(config);
        try {
            ProjectOptions options = new ProjectOptions();
            options.setEncryptionKey("TestEncryptionKey".getBytes());

            Project project = null;
            try {
                project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY, options);

                BucketConfig bucketConfig = new BucketConfig();
                Set<String> expectedBuckets = new HashSet<>();
                for (int i = 0; i < 10; i++) {
                    String expectedBucket = "testBucket" + i;
                    project.createBucket(expectedBucket, bucketConfig);
                    expectedBuckets.add(expectedBucket);
                }

                BucketList bucketList = project.listBuckets("", 1, 100);
                assertEquals(false, bucketList.more());
                String aa = "";
                for(int i =0; i< bucketList.length();i++){
                    aa += bucketList.item(i).getName() +"|";
                }

                assertEquals(aa, expectedBuckets.size(), bucketList.length());

                for (String bucket : expectedBuckets){
                    project.deleteBucket(bucket);
                }

                bucketList = project.listBuckets("", 1, 100);
                assertEquals(false, bucketList.more());
                assertEquals(0, bucketList.length());
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
    public void testUploadDownloadInline() throws Exception {
        Config config = new Config();

        Uplink uplink = new Uplink(config);
        try {
            ProjectOptions options = new ProjectOptions();
            options.setEncryptionKey("TestEncryptionKey".getBytes());

            Project project = null;
            try {
                project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY, options);

                BucketAccess access = new BucketAccess();
                access.setPathEncryptionKey("TestEncryptionKey".getBytes());

                RedundancyScheme scheme = new RedundancyScheme();
                scheme.setRequiredShares((short)2);
                scheme.setRepairShares((short)4);
                scheme.setOptimalShares((short)6);
                scheme.setTotalShares((short)8);

                BucketConfig bucketConfig = new BucketConfig();
                bucketConfig.setRedundancyScheme(scheme);

                project.createBucket("test", bucketConfig);

                Bucket bucket = project.openBucket("test", access);

                byte[] expectedData = new byte[1024];
                Random random = new Random() ;
                random.nextBytes(expectedData);

                {
                    Writer writer = bucket.newWriter("object/path", new WriterOptions());
                    try {
                        writer.write(expectedData);
                    }catch(Exception e){
                        e.printStackTrace();
                    } finally {
                        writer.close();
                    }
                }

                {
                    Reader reader = bucket.newReader("object/path", new ReaderOptions());
                    try {
                        ByteArrayOutputStream writer = new ByteArrayOutputStream();
                        byte[] buf = new byte[256];
                        int read = 0;
                        while ((read = reader.read(buf)) != -1) {
                            writer.write(buf, 0, read);
                        }
                        assertArrayEquals(writer.toByteArray(), expectedData);
                    } finally {
                        reader.close();
                    }
                }

                project.deleteBucket("test");
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