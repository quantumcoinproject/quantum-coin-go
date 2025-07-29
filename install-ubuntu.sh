#!/bin/bash

apt-get install -qq -y pkg-config
apt-get install -qq -y unzip
apt-get update -qq -y

mkdir templibs
mkdir templibs/pkg-config

mkdir templibs/liboqs

chmod 777 $PWD/.config/template/*

cp $PWD/.config/template/liboqs-template.pc $PWD/templibs/pkg-config/liboqs.pc

sed -i -e "s|\[INCLUDE_DIR\]|$PWD/templibs/liboqs/build/include|" $PWD/templibs/pkg-config/liboqs.pc
sed -i -e "s|\[LIB_DIR\]|$PWD/templibs/liboqs|" $PWD/templibs/pkg-config/liboqs.pc

curl -Lo $PWD/templibs/liboqs/includes.zip https://github.com/DogeProtocol/liboqs/releases/download/v0.0.4/includes.zip
unzip $PWD/templibs/liboqs/includes.zip -d $PWD/templibs/liboqs
echo "e04a39e332b169aad8370fbcd99aa8ab03ab5d0e621d711e78c6c9f6aa341d56 $PWD/templibs/liboqs/includes.zip" | sha256sum --check  - || exit 1

curl -Lo $PWD/templibs/liboqs/liboqs.so.5 https://github.com/DogeProtocol/liboqs/releases/download/v0.0.4/liboqs.so.5
echo "6694aaff32255faafab324011b7f5ea5ca0f527e0b901265597871dfb01ddf72 $PWD/templibs/liboqs/liboqs.so.5" | sha256sum --check  - || exit 1

curl -Lo $PWD/templibs/liboqs/liboqs.so https://github.com/DogeProtocol/liboqs/releases/download/v0.0.4/liboqs.so
echo "6694aaff32255faafab324011b7f5ea5ca0f527e0b901265597871dfb01ddf72 $PWD/templibs/liboqs/liboqs.so" | sha256sum --check  - || exit 1

echo " "
echo "Installation complete. To start building:"
echo "1) Switch to the go-dp folder."
echo "2) Set the following environment variable; you would want to add it to your bash profile."
echo " "
echo "   export PKG_CONFIG_PATH=$PWD/templibs/pkg-config"
echo " "
echo "3) Then run the following command: "
echo "4) go build -o YOUR_BUILD_DIR ./..."


