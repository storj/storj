FROM golang:1.13.1

ENV ANDROID_HOME /opt/android-sdk-linux
RUN apt-get update -qq

RUN dpkg --add-architecture i386
RUN apt-get update -qq
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y unzip openjdk-11-jdk rsync qemu-kvm qemu-utils libc6:i386 libstdc++6:i386 libgcc1:i386 libncurses5:i386 libz1:i386

RUN cd /opt \
    && wget -q https://dl.google.com/android/repository/sdk-tools-linux-4333796.zip -O android-sdk-tools.zip \
    && unzip -q android-sdk-tools.zip -d ${ANDROID_HOME} \
    && rm android-sdk-tools.zip

RUN cd ${ANDROID_HOME}/tools/bin \
    && mkdir jaxb_lib \
    && wget http://central.maven.org/maven2/javax/activation/activation/1.1.1/activation-1.1.1.jar -O jaxb_lib/activation.jar \
    && wget http://central.maven.org/maven2/javax/xml/jaxb-impl/2.1/jaxb-impl-2.1.jar -O jaxb_lib/jaxb-impl.jar \
    && wget http://central.maven.org/maven2/org/glassfish/jaxb/jaxb-xjc/2.3.2/jaxb-xjc-2.3.2.jar -O jaxb_lib/jaxb-xjc.jar \
    && wget http://central.maven.org/maven2/org/glassfish/jaxb/jaxb-core/2.3.0.1/jaxb-core-2.3.0.1.jar -O jaxb_lib/jaxb-core.jar \
    && wget http://central.maven.org/maven2/org/glassfish/jaxb/jaxb-jxc/2.3.2/jaxb-jxc-2.3.2.jar -O jaxb_lib/jaxb-jxc.jar \
    && wget http://central.maven.org/maven2/javax/xml/bind/jaxb-api/2.3.1/jaxb-api-2.3.1.jar -O jaxb_lib/jaxb-api.jar

RUN export JAXB=${ANDROID_HOME}/tools/bin/jaxb_lib/activation.jar:${ANDROID_HOME}/tools/bin/jaxb_lib/jaxb-impl.jar:${ANDROID_HOME}/tools/bin/jaxb_lib/jaxb-xjc.jar:${ANDROID_HOME}/tools/bin/jaxb_lib/jaxb-core.jar:${ANDROID_HOME}/tools/bin/jaxb_lib/jaxb-jxc.jar:${ANDROID_HOME}/tools/bin/jaxb_lib/jaxb-api.jar \
    && sed -i '/^eval set -- $DEFAULT_JVM_OPTS.*/i CLASSPATH='$JAXB':$CLASSPATH' ${ANDROID_HOME}/tools/bin/sdkmanager

ENV PATH ${PATH}:${ANDROID_HOME}/tools:${ANDROID_HOME}/tools/bin:${ANDROID_HOME}/platform-tools
 
# accept all licenses
RUN yes | sdkmanager  --licenses
RUN touch /root/.android/repositories.cfg

# Platform tools
RUN sdkmanager "platform-tools" "platforms;android-24" "tools" "emulator"

# The `yes` is for accepting all non-standard tool licenses.
RUN yes | sdkmanager --update --channel=3
RUN yes | sdkmanager \
    "ndk-bundle" \
    "system-images;android-24;default;x86_64"