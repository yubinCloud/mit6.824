# MIT 6.824 的 Lab

不要看不要看，这 lab 的灵魂就是去尝试探索~

## Wiki

[MIT6.824 Wiki](https://github.com/yubinCloud/mit6.824/wiki)

## 环境搭建

由于 lab 必须在 Linux 下运行，因此可以开启一个 docker，并全程在 docker 内完成。

首先下载 docker image：

```shell
docker pull golang:1.17.6-buster
```

然后将其运行为一个持续运行的容器：

```shell
docker -it run --name mit6.824 -d golang:1.17.6-buster /bin/bash
```

之后可以通过 VS Code 的 remote 功能连接到这个容器之中，再在容器内下载一下 git，把代码克隆到容器之中就可以开始做实验了。

### 网络问题

为了使用 VSCode 的智能提示的功能，VSCode 会自动下载相关的 go 工具，但由于网络问题很容易失败，可以通过如下配置来解决：

```shell
go env -w GO111MODULE=on
go env -w GOPROXY=https://goproxy.cn
```

之后重启便可成功下载。

### gopls 版本问题

该 lab 所需的 golang 版本为 1.17.6，但 VSCode 所下载的最新的 gopls 版本与之不匹配，可以通过更换 gopls 版本的问题来解决：

```shell
go install golang.org/x/tools/gopls@v0.11.0

gopls version
```
