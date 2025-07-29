#!/bin/bash

brew install pkg-config --debug --verbose
brew install coreutils --debug --verbose

mkdir templibs
mkdir templibs/pkg-config

mkdir templibs/liboqs

chmod 777 $PWD/.config/template/*

cp $PWD/.config/template/liboqs-template.pc $PWD/templibs/pkg-config/liboqs.pc

sed -i -e "s|\[INCLUDE_DIR\]|$PWD/templibs/liboqs/build/include|" $PWD/templibs/pkg-config/liboqs.pc
sed -i -e "s|\[LIB_DIR\]|$PWD/templibs/liboqs|" $PWD/templibs/pkg-config/liboqs.pc

curl -Lo $PWD/templibs/liboqs/includes.zip https://github.com/DogeProtocol/liboqs/releases/download/v0.0.7/includes.zip
unzip $PWD/templibs/liboqs/includes.zip -d $PWD/templibs/liboqs
echo "74d465d7024c387b28981ce4f5e14657a2b8d224bfc8cde7474a230e4ea3a5e1 $PWD/templibs/liboqs/includes.zip" | sha256sum --check  - || exit 1

curl -Lo $PWD/templibs/liboqs/liboqs.5.dylib https://github.com/DogeProtocol/liboqs/releases/download/v0.0.7/liboqs.5.dylib
echo "fc82da9db59eab54de0da416df8e13d185b6d2db0a04a31b6ba013525ddf09c5 $PWD/templibs/liboqs/liboqs.5.dylib" | sha256sum --check  - || exit 1

curl -Lo $PWD/templibs/liboqs/liboqs.dylib https://github.com/DogeProtocol/liboqs/releases/download/v0.0.7/liboqs.dylib
echo "fc82da9db59eab54de0da416df8e13d185b6d2db0a04a31b6ba013525ddf09c5 $PWD/templibs/liboqs/liboqs.dylib" | sha256sum --check  - || exit 1

echo " "
echo "Installation complete. To start building:"
echo "1) Switch to the go-dp folder."
echo "2) Set the following environment variable; you would want to add it to your shell profile."
echo " "
echo "   export PKG_CONFIG_PATH=$PWD/templibs/pkg-config"
echo " "
echo "3) Then run the following command: "
echo "4) go build -o YOUR_BUILD_DIR ./..."


