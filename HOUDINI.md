# Houdini

## Config

```
$ cat /etc/docker/houdini.ini
[default]
user=1000:1000
environment=LD_LIBRARY_PATH=/usr/lib/nvidia,PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/lib/nvidia/bin
mounts=/home:/home,/usr/lib/nvidia-384:/usr/lib/nvidia
devices=/dev/nvidia0,/dev/nvidiactl,/dev/nvidia-uvm

[user]
mode=static

[cuda]
libcuda=/usr/lib/x86_64-linux-gnu/
```

```
$ docker run -it -p 8888:8888 gcr.io/tensorflow/tensorflow:latest-gpu
```

Houdini manipulates API payload.

```
HOUDINI: Add bind '/home:/home'
HOUDINI: Add bind '/usr/lib/nvidia-384:/usr/lib/nvidia'
HOUDINI: Add cuda library '/usr/lib/x86_64-linux-gnu/libcuda.so:/usr/lib/x86_64-linux-gnu/libcuda.so'
HOUDINI: Add cuda library '/usr/lib/x86_64-linux-gnu/libcuda.so.1:/usr/lib/x86_64-linux-gnu/libcuda.so.1'
HOUDINI: Add cuda library '/usr/lib/x86_64-linux-gnu/libcuda.so.384.111:/usr/lib/x86_64-linux-gnu/libcuda.so.384.111'
HOUDINI: Add device '/dev/nvidia0'
HOUDINI: Add device '/dev/nvidiactl'
HOUDINI: Add device '/dev/nvidia-uvm'
HOUDINI: Add env 'LD_LIBRARY_PATH=/usr/lib/nvidia'
HOUDINI: Add env 'PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/lib/nvidia/bin'
```