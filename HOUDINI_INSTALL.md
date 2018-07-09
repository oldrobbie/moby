## Houdini Install

In order to run the engine,
- the binaries have to be placed on the host
- the houdini configuration has to be created
- the startup script needs to pick up the correct binaries (`PATH`)

### Common
After building the houdini engine `VERSION=houdini-5 make` copy the file onto the server.

##### Preparing target directory on the host
```bash
docker@christiankniep-testkit-EEAF47-ubuntu-0:~$ mkdir binary-daemon
```

##### Install driver

**Ubuntu**
```bash
$ sudo apt-get install -y nvidia-384
```

##### Sync Files

```bash
$ rsync -iP bundles/binary-daemon/. docker@ww.xx.yy.zz:./binary-daemon/.
```

##### Create houdini config

```bash
$ echo """[default]
debug=true
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
cuda-libpath=/usr/lib/x86_64-linux-gnu/""" |sudo tee /etc/docker/houdini.ini
```

### SystemD Unitfile

To pick up the correct `dockerd` from the copied location, change the `ExecStart` line to use a relative path and add `Environment`, so that `PATH` points to the copied location first.
The location of the unit file can be seen in the `sudo systemctl status docker` output.

```
$ sudo vim /lib/systemd/system/docker.service
#ExecStart=/usr/bin/dockerd -H fd:// -H unix:// -H tcp://0.0.0.0:2376
Environment=PATH=/home/docker/binary-daemon:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ExecStart=dockerd -H fd:// -H unix:// -H tcp://0.0.0.0:2376
```

Reload the daemon script.
```bash
$ systemctl daemon-reload
```



### Debug

```bash
$ sudo systemctl status -l -n500 docker |awk -Fmsg= '/HOUDINI/{print $2}'
"HOUDINI: Overwrite user '' with '1000:1000'"
"HOUDINI: Found env 'DATA=/data'"
"HOUDINI: Add env 'DATA=/data'"
"HOUDINI: Add bind '/home:/home'"
"HOUDINI: Add GPU, as env 'HOUDINI_GPU_ENABLED' is 'true'."
"HOUDINI: Add GPU bind mount '/usr/lib/nvidia-384:/usr/lib/nvidia:ro'"
"HOUDINI: Add GPU bind mount '/usr/bin/nvidia-smi:/usr/bin/nvidia-smi:ro'"
"HOUDINI: Found env 'LD_LIBRARY_PATH=/usr/lib/nvidia'"
"HOUDINI: Add env 'LD_LIBRARY_PATH=/usr/lib/nvidia'"
"HOUDINI: Found env 'NVIDIA_VISIBLE_DEVICES=all'"
"HOUDINI: Add env 'NVIDIA_VISIBLE_DEVICES=all'"
"HOUDINI: Search dir '/usr/lib/x86_64-linux-gnu/' for 'libcuda'"
"HOUDINI: Add cuda library '/usr/lib/x86_64-linux-gnu/libcuda.so:ro"
"HOUDINI: Add cuda library '/usr/lib/x86_64-linux-gnu/libcuda.so.1:ro"
"HOUDINI: Add cuda library '/usr/lib/x86_64-linux-gnu/libcuda.so.384.130:ro"
"HOUDINI: Add device '/dev/nvidia0'"
"HOUDINI: Add device '/dev/nvidia-uvm'"
"HOUDINI: Add device '/dev/nvidiactl'"
```