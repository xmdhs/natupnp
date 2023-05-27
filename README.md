# natupnp
又一个 tcp 打洞工具，参考了 [Natter](https://github.com/MikeWang000000/Natter) 和 [natmap](https://github.com/heiher/natmap)

也就是能在 nat1 的情况，在没有公网 ip 时，打开公网 ip 的 tcp 端口，可以让其他人直接访问到。

和以上两个工具相同，不过不同的是，这个工具集成了 upnp 的功能。能在路由上设置端口映射，增加打洞的成功率。

## 使用示例
首先可以检测当前网络是否支持打洞。

`natupnp -t` 将会在 8086 上监听，然后打印公网地址和端口，可以尝试使用其他网络用浏览器访问，如果显示 ok 则说明打洞成功。

## 防火墙以及路由器的配置
对于运行 natupnp 的主机，需要开放 -p 指定的端口， `-p 8080` 8080 就是需要开放的端口了。

对于路由器，分三种情况。

### 光猫拨号，网线插在路由器的 lan 口
这种情况下，光猫管理界面的 ip，和连接路由器的主机的 ip 在一个网段内。

此时，需要在光猫上开启 upnp。一般在 高级配置-日常应用-upnp 配置，可能需要光猫的超级管理员密码，通常能在网上搜到。

### 光猫拨号，网线插在路由器的 wan 口
这种情况下的光猫管理界面 ip，和连接路由器的设备的 ip 不在一个网段。

在光猫上设置 DMZ 主机，通常在 高级配置-NAT设置-DMZ配置，在其中配置路由器的 ip 或者 mac 地址，并把路由器的 upnp 功能开启（通常默认是开启）

### 光猫桥接，路由器拨号
应该不需要额外的操作即可成功打洞，若不成功，可以检查 upnp 功能是否开启。

## 命令
和 natmap 一样，支持绑定和转发两种模式。绑定模式在 linux 上似乎有一些问题，建议用转发。


### 绑定
`natupnp -p 8080`

当前设备需在防火墙开放 8080 端口

### 转发
`natupnp -p 8080 -d 127.0.0.1:80`

会将数据转发到 127.0.0.1:80，也需要在防火墙开放 8080 端口

若成功会打印在公网 ip 上开放的端口和公网 ip。