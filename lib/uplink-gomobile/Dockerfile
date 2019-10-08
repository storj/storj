FROM golang:1.13.1-stretch

ENV ANDROID_HOME /opt/android-sdk-linux

RUN apt-get update -qq

# Dependencies to execute Android builds
RUN dpkg --add-architecture i386
RUN apt-get update -qq
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y unzip openjdk-8-jdk rsync libc6:i386 libstdc++6:i386 libgcc1:i386 libncurses5:i386 libz1:i386

RUN cd /opt \
    && wget -q https://dl.google.com/android/repository/sdk-tools-linux-4333796.zip -O android-sdk-tools.zip \
    && unzip -q android-sdk-tools.zip -d ${ANDROID_HOME} \
    && rm android-sdk-tools.zip

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