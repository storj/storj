package io.storj.example;

import android.support.v7.app.AppCompatActivity;
import android.os.Bundle;
import mobile.*;

// remember to add into Android project manifest
//
//    <uses-permission android:name="android.permission.INTERNET" />
//    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />


public class MainActivity extends AppCompatActivity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        Config config = new Config();
        config.setIdentity(null);

        try {
            Uplink uplink = new Uplink(config);
            try {
                ProjectOptions options = new ProjectOptions();
                options.setEncryptionKey("TestEncryptionKey".getBytes());

                // 10.0.2.2 refers to the host running the android simulator
                Project project = uplink.openProject("10.0.2.2:10000", "QQPJHE6DTDDU7H1OM2CF2L5O3LVQ231PE3CA490=", options);
                try {
                    BucketAccess access = new BucketAccess();
                    access.setPathEncryptionKey("TestEncryptionKey".getBytes());
                    Bucket bucket = project.openBucket("test", access);

                    Writer writer = bucket.newWriter("hello", new WriterOptions());
                    try {
                        writer.write("hello".getBytes());
                    } finally {
                        writer.close();
                    }

                    Reader reader = bucket.newReader("hello", new ReaderOptions());
                    try {
                        byte[] data = new byte[5];
                        reader.read(data);
                        System.out.println("---");
                        System.out.println(data.toString());
                        System.out.println("---");
                    } finally {
                        writer.close();
                    }

                } finally {
                    project.close();
                }
            } finally {
                uplink.close();
            }
        } catch(Exception e) {
            System.out.println(e);
        }
    }
}
