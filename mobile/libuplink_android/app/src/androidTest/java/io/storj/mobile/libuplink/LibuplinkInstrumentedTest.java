package io.storj.mobile.libuplink;

import android.support.test.InstrumentationRegistry;
import android.support.test.runner.AndroidJUnit4;

import org.junit.Assert;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

import java.io.ByteArrayOutputStream;
import java.util.HashSet;
import java.util.Random;
import java.util.Set;

import io.storj.libuplink.mobile.*;

import static org.junit.Assert.*;

@RunWith(AndroidJUnit4.class)
public class LibuplinkInstrumentedTest {

    public static final String VALID_SATELLITE_ADDRESS = InstrumentationRegistry.getArguments().getString("storj.sim.host", "192.168.8.134:10000");
    public static final String VALID_API_KEY = InstrumentationRegistry.getArguments().getString("api.key", "GBK6TEMIPJQUOVVN99C2QO9USKTU26QB6C4VNM0=");

    String filesDir;

    @Before
    public void setUp() {
        filesDir = InstrumentationRegistry.getTargetContext().getFilesDir().getAbsolutePath();
    }

    @Test
    public void testOpenProjectFail() throws Exception {
        Config config = new Config();

        Uplink uplink = new Uplink(config, filesDir);
        try {
            Project project = null;
            try {
                // 10.0.2.2 refers to not existing satellite
                project = uplink.openProject("10.0.2.2:1", VALID_API_KEY);
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

        Uplink uplink = new Uplink(config, filesDir);
        try {
            Project project = null;
            try {
                project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY);

                String expectedBucket = "test-bucket";
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

        Uplink uplink = new Uplink(config, filesDir);
        try {
            Project project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY);
            try {
                BucketConfig bucketConfig = new BucketConfig();
                Set<String> expectedBuckets = new HashSet<>();
                for (int i = 0; i < 10; i++) {
                    String expectedBucket = "test-bucket" + i;
                    project.createBucket(expectedBucket, bucketConfig);
                    expectedBuckets.add(expectedBucket);
                }

                BucketList bucketList = project.listBuckets("", 1, 100);
                assertEquals(false, bucketList.more());
                String aa = "";
                for (int i = 0; i < bucketList.length(); i++) {
                    aa += bucketList.item(i).getName() + "|";
                }

                assertEquals(aa, expectedBuckets.size(), bucketList.length());

                for (String bucket : expectedBuckets) {
                    project.deleteBucket(bucket);
                }

                bucketList = project.listBuckets("", 1, 100);
                assertEquals(false, bucketList.more());
                assertEquals(0, bucketList.length());
            } finally {
                project.close();
            }
        } finally {
            uplink.close();
        }
    }

    @Test
    public void testUploadDownloadInline() throws Exception {
        Config config = new Config();

        Uplink uplink = new Uplink(config, filesDir);
        try {
            Project project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY);
            try {
                EncryptionAccess access = new EncryptionAccess();
                access.setDefaultKey("TestEncryptionKey".getBytes());

                RedundancyScheme scheme = new RedundancyScheme();
                scheme.setRequiredShares((short) 4);
                scheme.setRepairShares((short) 6);
                scheme.setOptimalShares((short) 8);
                scheme.setTotalShares((short) 10);

                BucketConfig bucketConfig = new BucketConfig();
                bucketConfig.setRedundancyScheme(scheme);

                project.createBucket("test", bucketConfig);

                Bucket bucket = project.openBucket("test", access);

                byte[] expectedData = new byte[1024];
                Random random = new Random();
                random.nextBytes(expectedData);

                {
                    Writer writer = bucket.newWriter("object/path", new WriterOptions());
                    try {
                        writer.write(expectedData, 0, expectedData.length);
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

                bucket.close();

                project.deleteBucket("test");
            } finally {
                project.close();
            }
        } finally {
            uplink.close();
        }
    }


    @Test
    public void testUploadDownloadDeleteRemote() throws Exception {
        Config config = new Config();

        Uplink uplink = new Uplink(config, filesDir);
        try {
            Project project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY);
            try {
                EncryptionAccess access = new EncryptionAccess();
                access.setDefaultKey("TestEncryptionKey".getBytes());

                RedundancyScheme scheme = new RedundancyScheme();
                scheme.setRequiredShares((short) 4);
                scheme.setRepairShares((short) 6);
                scheme.setOptimalShares((short) 8);
                scheme.setTotalShares((short) 10);

                BucketConfig bucketConfig = new BucketConfig();
                bucketConfig.setRedundancyScheme(scheme);

                project.createBucket("test", bucketConfig);

                Bucket bucket = project.openBucket("test", access);

                byte[] expectedData = new byte[1024 * 100];
                Random random = new Random();
                random.nextBytes(expectedData);
                {
                    Writer writer = bucket.newWriter("object/path", new WriterOptions());
                    try {
                        writer.write(expectedData, 0, expectedData.length);
                    } finally {
                        writer.close();
                    }
                }

                {
                    Reader reader = bucket.newReader("object/path", new ReaderOptions());
                    try {
                        ByteArrayOutputStream writer = new ByteArrayOutputStream();
                        byte[] buf = new byte[4096];
                        int read = 0;
                        while ((read = reader.read(buf)) != -1) {
                            writer.write(buf, 0, read);
                        }
                        assertEquals(expectedData.length, writer.size());
                    } finally {
                        reader.close();
                    }
                }

                bucket.deleteObject("object/path");

                try {
                    bucket.deleteObject("object/path");
                } catch (Exception e) {
                    assertTrue(e.getMessage().startsWith("object not found"));
                }

                bucket.close();

                project.deleteBucket("test");
            } finally {
                project.close();
            }
        } finally {
            uplink.close();
        }
    }

    @Test
    public void testListObjects() throws Exception {
        Config config = new Config();

        Uplink uplink = new Uplink(config, filesDir);
        try {
            Project project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY);

            try {
                EncryptionAccess access = new EncryptionAccess();
                access.setDefaultKey("TestEncryptionKey".getBytes());

                BucketConfig bucketConfig = new BucketConfig();
                bucketConfig.setRedundancyScheme(new RedundancyScheme());

                BucketInfo bucketInfo = project.createBucket("test-bucket", bucketConfig);
                assertEquals("test-bucket", bucketInfo.getName());

                Bucket bucket = project.openBucket("test-bucket", access);

                long before = System.currentTimeMillis();

                for (int i = 0; i < 13; i++) {
                    Writer writer = bucket.newWriter("path" + i, new WriterOptions());
                    try {
                        byte[] buf = new byte[0];
                        writer.write(buf, 0, buf.length);
                    } finally {
                        writer.close();
                    }
                }

                ListOptions listOptions = new ListOptions();
                listOptions.setCursor("");
                listOptions.setDirection(Mobile.DirectionForward);
                listOptions.setLimit(20);

                ObjectList list = bucket.listObjects(listOptions);
                assertEquals(13, list.length());

                for (int i = 0; i < list.length(); i++) {
                    ObjectInfo info = list.item(i);
                    assertEquals("test-bucket", info.getBucket());
                    assertTrue(info.getCreated() >= before);

                    // cleanup
                    bucket.deleteObject("path" + i);
                }

                bucket.close();

                project.deleteBucket("test-bucket");
            } finally {
                project.close();
            }
        } finally {
            uplink.close();
        }
    }

    @Test
    public void testEncryptionAccessFromPassphrase() throws Exception {
        Config config = new Config();

        Uplink uplink = new Uplink(config, filesDir);
        try {
            Project project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY);
            try {
                byte[] saltedKey = project.saltedKeyFromPassphrase("some-passphrase");
                EncryptionAccess ea = new EncryptionAccess(saltedKey);
                String serialized = ea.serialize();
                assertNotEquals("", serialized);
            } finally {
                project.close();
            }
        } finally {
            uplink.close();
        }
    }

    @Test
    public void testEncryptionAccessWithRoot() throws Exception {
        Config config = new Config();

        Uplink uplink = new Uplink(config, filesDir);
        try {
            Project project = uplink.openProject(VALID_SATELLITE_ADDRESS, VALID_API_KEY);
            try {
                byte[] saltedKey = project.saltedKeyFromPassphrase("some-passphrase");
                EncryptionAccess ea = Mobile.newEncryptionAccessWithRoot("bucket", "unencryptedPath", "encryptedPath", saltedKey);
                String serialized = ea.serialize();
                assertNotEquals("", serialized);
            } finally {
                project.close();
            }
        } finally {
            uplink.close();
        }
    }

    @Test
    public void testApiKey() throws Exception {
        String apiKeyData = "13YqeKQiA3ANSuDu4rqX6eGs3YWox9GRi9rEUKy1HidXiNNm6a5SiE49Hk9gomHZVcQhq4eFQh8yhDgfGKg268j6vqWKEhnJjFPLqAP";
        APIKey apiKey = Mobile.parseAPIKey(apiKeyData);

        String serialized = apiKey.serialize();
        assertEquals(serialized, apiKeyData);

        Caveat caveat = new Caveat();
        caveat.setDisallowDeletes(true);
        caveat.setDisallowWrites(true);
        caveat.setDisallowReads(true);
        caveat.setDisallowLists(true);
        caveat.setNotAfter(100);
        caveat.setNotBefore(50);

        CaveatPath path = new CaveatPath();
        path.setBucket("bucket".getBytes());
        path.setEncryptedPathPrefix("123456".getBytes());
        caveat.addCaveatPath(path);

        APIKey newAPIKey = apiKey.restrict(caveat);
        assertNotEquals("", newAPIKey.serialize());
    }


}
