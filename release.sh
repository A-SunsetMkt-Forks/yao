#!/bin/bash

repace() {
    echo $1
    sed -E "$1" ~/Code/yao/xiang/shell/install.sh > ~/Code/yao/xiang/shell/install.sh.new
}

# 关闭代理
all_proxy=""
http_proxy=""
https_proxy=""

VERSION=$(go run . version)
ssh max@demo-crm.iqka.com mkdir -p /data/demo-crm/ui/releases/$VERSION/
ssh max@demo-crm.iqka.com "echo {\\\"version\\\":\\\"$VERSION\\\"} > /data/demo-crm/ui/releases/latest.json"
scp ~/Code/bin/xiang-$VERSION-linux-amd64 max@demo-crm.iqka.com:/data/demo-crm/ui/releases/$VERSION/yao-$VERSION-linux-amd64
scp ~/Code/bin/xiang-$VERSION-darwin-amd64 max@demo-crm.iqka.com:/data/demo-crm/ui/releases/$VERSION/yao-$VERSION-darwin-amd64
# scp ~/Code/bin/xiang-$VERSION-windows-386 max@demo-crm.iqka.com:/data/demo-crm/ui/releases/$VERSION/yao-$VERSION-windows-386

repace "s/[0-9]+\.[0-9]+\.[0-9]+/$VERSION/g"
scp ~/Code/yao/xiang/shell/install.sh.new max@demo-crm.iqka.com:/data/demo-crm/ui/releases/$VERSION/install.sh
rm ~/Code/yao/xiang/shell/install.sh.new
