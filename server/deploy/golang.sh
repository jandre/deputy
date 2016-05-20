#!/bin/bash

GOLANG_VERSION=1.5.1 
GOLANG_SRC=https://storage.googleapis.com/golang/go$GOLANG_VERSION.linux-amd64.tar.gz 
cd 
wget -q $GOLANG_SRC 
tar -xvf go$GOLANG_VERSION.linux-amd64.tar.gz 
mv go go$GOLANG_VERSION 
mkdir -p $HOME/projects/go/{bin,pkg,src} 
ln -s go$GOLANG_VERSION go 
echo export PATH="$PATH:$HOME/go/bin" >> ~/.profile 
echo export GOROOT=$HOME/go/ >> ~/.profile 
echo export GOPATH=$HOME/projects/go >> ~/.profile 
echo export GOBIN=$HOME/projects/go/bin >> ~/.profile 

