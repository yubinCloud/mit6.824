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
docker run --name mit6.824 -d golang:1.17.6-buster /bin/bash
```

之后可以通过 VS Code 的 remote 功能连接到这个容器之中，再在容器内下载一下 git，把代码克隆到容器之中就可以开始做实验了。
