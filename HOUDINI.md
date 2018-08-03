# Houdini

## Config

```
$ cat /etc/docker/houdini.ini
[default]
debug=false
# If true, apply houdini onto each container
force-houdini=false
# trigger-label defines the label to trigger houdini (in case force-houdini is false)
trigger-label=houdini
# trigger environment variable (in case force-houdini is false)
trigger-env=HOUDINI_ENABLED
# Mounts to be applied
mounts=/home:/home
# Additional ENV - if the key already exists it won't be overwritten (see force-environment), semicolon separated
environment=DATA=/data
# Overwrite ENV if key already exists
force-environment=false
# /dev/nvidiactl,/dev/nvidia-uvm will be added in case nvidia devices are found
#devices=/dev/nvidia0
[container]
# if label is set use the value
remove-label=houdini.container.remove
# Removes the container automatically once finished
remove=true

[user]
keep-user-label=houdini.user.keep
mode=static
default=1000:1000

[gpu]
# Apply GPU enablement to all houdini containers (use in conjunction with force-houdini=true)
force=false
# ENV to trigger GPU (if force=false)
trigger-env=HOUDINI_GPU_ENABLED
# label to trigger GPU (if force=false)
trigger-label=houdini-gpu-enabled
# Mounts specific to GPU
mounts=/usr/lib/nvidia-384:/usr/lib/nvidia:ro,/usr/bin/nvidia-smi:/usr/bin/nvidia-smi:ro
# Additional ENV - if the key already exists it won't be overwritten (see force-environment), semicolon separated
environment=LD_LIBRARY_PATH=/usr/lib/nvidia|NVIDIA_VISIBLE_DEVICES=all
# Overwrite ENV if key already exists
force-environment=true
# What file prefix to look out for (comma separated list)
cuda-files=libcuda
# Where to look for those files
cuda-libpath=/usr/lib/x86_64-linux-gnu/
```


## Keras AI

```
$ id docker
uid=1000(docker) gid=1000(docker) groups=1000(docker)
$ docker run -ti --rm --label houdini.enable=true qnib/keras python mnist_cnn.py /home/docker/
Using TensorFlow backend.
Downloading data from https://s3.amazonaws.com/img-datasets/mnist.npz
11493376/11490434 [==============================] - 2s 0us/step
```

Since the `/home/` directory is mounted, the second time, the data-set is already there.

```
$ docker run -ti --rm --label houdini.enable=true qnib/keras python mnist_cnn.py /home/docker/
Using TensorFlow backend.
x_train shape: (60000, 28, 28, 1)
60000 train samples
10000 test samples
Train on 60000 samples, validate on 10000 samples
Epoch 1/12
2018-06-05 11:46:32.968603: I tensorflow/core/platform/cpu_feature_guard.cc:140] Your CPU supports instructions that this TensorFlow binary was not compiled to use: AVX2 FMA
2018-06-05 11:46:35.815165: I tensorflow/stream_executor/cuda/cuda_gpu_executor.cc:898] successful NUMA node read from SysFS had negative value (-1), but there must be at least one NUMA node, so returning NUMA node zero
2018-06-05 11:46:35.815525: I tensorflow/core/common_runtime/gpu/gpu_device.cc:1212] Found device 0 with properties:
name: Tesla K80 major: 3 minor: 7 memoryClockRate(GHz): 0.8235
```

And by forcing the `UID;GID` (`HOUDINI: Overwrite user 'ubuntu:ubuntu' with '1000:1000'"`, the container is only able to use the file-system like the user outside of the container is able to.

```
$ docker run -ti --rm --label houdini.enable=true --user=ubuntu:ubuntu ubuntu touch /home/ubuntu/test
  touch: cannot touch '/home/ubuntu/test': Permission denied
```

# Configuration
## Default
In this section default mounts, environment variables (added not overwritten) and devices can be set.
```
[default]
mounts=/home:/home,/usr/lib/nvidia-384:/usr/lib/nvidia
environment=LD_LIBRARY_PATH=/usr/lib/nvidia
devices=/dev/nvidia0,/dev/nvidiactl,/dev/nvidia-uvm
```
## user
To influence the `--user` config, this section can be used.

```
[user]
mode=static
user=1000:1000
key=HOUDINI_USER
```

The following modes are available.

- `static` passes the `user` config along.
- `default` evaluates the `user` config on the host system (**unstable**)
- `env` fetches the config from the containers Environment `key` (default `HOUDINI_USER`).

## cuda
```
[cuda]
files=libcuda
libpath=/usr/lib/x86_64-linux-gnu/
```

As the nvidia driver is not enough, some libraries are mapped in. In case `libpath` is set all files with the prefix provided by `files` will be mapped in read-only.


### NVIDIA_VISIBLE_DEVICES
Like with [nvidia-docker](https://github.com/nvidia/nvidia-container-runtime#nvidia_visible_devices), the plugin will detect `NVIDIA_VISIBLE_DEVICES
` and act accordingly; even though only on `all` or a comma-separated list of integers.

Either within the `houdini.ini` file or as an environment variable passed to the container.


```
$ docker run -ti --rm -e HOUDINI_GPU_ENABLED=true -e NVIDIA_VISIBLE_DEVICES=all qnib/keras python mnist_cnn.py /home/docker/
```
