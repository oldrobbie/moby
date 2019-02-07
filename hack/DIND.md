# Test platform.Feature using Docker-in-Docker

## Dockerfile

The `Dockerfile` can be found in [qnib/uplain-dind](https://github.com/qnib/uplain-dind).
**Root-less** Since vpnkit is not building ([#455](https://github.com/moby/vpnkit/issues/455)) I removed vpnkit from being installed.


```
FROM qnib/uplain-dind:2019-02-01.1

RUN apt-get update \
 && apt-get install -y jq moreutils

COPY binary-daemon/docker-init \
     binary-daemon/docker-proxy \
     binary-daemon/ctr \
     binary-daemon/containerd \
     binary-daemon/runc \
     binary-daemon/containerd-shim \
     binary-daemon/dockerd-rootless.sh \
     binary-daemon/rootlesskit \
     /usr/bin/
COPY binary-daemon/dockerd-dev /usr/bin/dockerd
RUN mkdir -p /root/.docker/ \
 && echo '{"experimental": "enabled"}' > /root/.docker/config.json
RUN jq '.experimental=true' /etc/docker/daemon.json |sponge /etc/docker/daemon.json
```

## Change PATH

Since the directory `./bundles` is present in the `.dockerignore` blacklist, the `docker build` command needs to be executed within the blacklisted subdirectory.

```
$ echo """FROM qnib/uplain-dind:2019-02-01.1

  RUN apt-get update \
   && apt-get install -y jq moreutils

  COPY binary-daemon/docker-init \
       binary-daemon/docker-proxy \
       binary-daemon/ctr \
       binary-daemon/containerd \
       binary-daemon/runc \
       binary-daemon/containerd-shim \
       binary-daemon/dockerd-rootless.sh \
       binary-daemon/rootlesskit \
       /usr/bin/
  COPY binary-daemon/dockerd-dev /usr/bin/dockerd
  RUN mkdir -p /root/.docker/ \
   && echo '{"experimental": "enabled"}' > /root/.docker/config.json
  RUN jq '.experimental=true' /etc/docker/daemon.json |sponge /etc/docker/daemon.json""" |docker build -t moby:dind -f=- ./bundles
Sending build context to Docker daemon  171.5MB
Step 1/6 : FROM qnib/uplain-dind:2019-02-01.1
 ---> a108318e2cd5
Step 2/6 : RUN apt-get update  && apt-get install -y jq moreutils
 ---> Using cache
 ---> 38398f980f93
Step 3/6 : COPY binary-daemon/docker-init      binary-daemon/docker-proxy      binary-daemon/ctr      binary-daemon/containerd      binary-daemon/runc      binary-daemon/containerd-shim      binary-daemon/dockerd-rootless.sh      binary-daemon/rootlesskit      /usr/bin/
 ---> Using cache
 ---> 76c0385336af
Step 4/6 : COPY binary-daemon/dockerd-dev /usr/bin/dockerd
 ---> Using cache
 ---> 3a46919ab405
Step 5/6 : RUN mkdir -p /root/.docker/  && echo '{"experimental": "enabled"}' > /root/.docker/config.json
 ---> Using cache
 ---> f48a6d946610
Step 6/6 : RUN jq '.experimental=true' /etc/docker/daemon.json |sponge /etc/docker/daemon.json
 ---> Using cache
 ---> d287f9cfb8ab
Successfully built d287f9cfb8ab
Successfully tagged moby:dind
```

## Deploy and test

Using Kubernetes (via Docker Desktop, Minikube or a Kubernetes cluster), we can deploy the freshly build image.

```
$ echo """
apiVersion: v1
kind: Pod
metadata:
 name: dind
spec:
 containers:
   - name: docker
     image: moby:dind
     command:
       - "tail"
       - "-f"
       - "/dev/null"
     securityContext:
       privileged: true""" | kubectl apply -f -
pod "dind" created
```

Now we start `dockerd` specifying platform features.

```
$ docker exec -ti $(docker ps -ql) dockerd --debug --platform-feature=test2
WARN[2019-02-07T10:10:15.499610900Z] Running experimental build
DEBU[2019-02-07T10:10:15.502976700Z] Listener created for HTTP on unix (/var/run/docker.sock)
WARN[2019-02-07T10:10:15.503130800Z] [!] DON'T BIND ON ANY IP ADDRESS WITHOUT setting --tlsverify IF YOU DON'T KNOW WHAT YOU'RE DOING [!]
DEBU[2019-02-07T10:10:15.505090300Z] Listener created for HTTP on tcp (0.0.0.0:2376)
*snip*
DEBU[2019-02-07T10:10:15.628135300Z] platform.features to pick images from ManifestLists: [test2]
*snip*
```

Downloding an image via ManifestList ([qnib/plain-manifestlist](https://github.com/qnib/plain-manifestlist))...

```
docker exec -ti $(docker ps -ql) docker run --rm -ti  qnib/plain-manifestlist
Unable to find image 'qnib/plain-manifestlist:latest' locally
latest: Pulling from qnib/plain-manifestlist
6c40cc604d8e: Pull complete
1df22ac9cfa2: Pull complete
Digest: sha256:09d55c488ff78bbefa3a76728baab1f2dad78f4f07e7b8155ec6a84d81698436
Status: Downloaded newer image for qnib/plain-manifestlist:latest
test2
```

... triggers the docker daemon to take features into consideration when matching a ManifestList.

```
DEBU[2019-02-07T10:10:57.298998200Z] Create platform spec with Features: [test2]
DEBU[2019-02-07T10:10:57.299260900Z] Trying to pull qnib/plain-manifestlist from https://registry-1.docker.io v2
DEBU[2019-02-07T10:10:59.220693200Z] Pulling ref from V2 registry: qnib/plain-manifestlist:latest
DEBU[2019-02-07T10:10:59.220989600Z] docker.io/qnib/plain-manifestlist:latest resolved to a manifestList object with 5 entries; looking for a linux/amd64/amd64 match
DEBU[2019-02-07T10:10:59.228775000Z] m.Features: [test2] | [test1] platform.Features
DEBU[2019-02-07T10:10:59.230477200Z] m.Features: [test2] | [test2] platform.Features
DEBU[2019-02-07T10:10:59.231988900Z] found match for linux/amd64 with media type application/vnd.docker.distribution.manifest.v2+json, digest sha256:3ab7d50dc8b9677e013d286dd8ed6e4e91f331b2d782f73dac5910cee497886d
DEBU[2019-02-07T10:10:59.232232500Z] m.Features: [test2] | [test3] platform.Features
DEBU[2019-02-07T10:10:59.232814200Z] m.Features: [test2] | [test4] platform.Features
DEBU[2019-02-07T10:10:59.233571100Z] m.Features: [test2] | [test5] platform.Features
```

#### Multiple Features

In order to optizmize images for different aspects (Skylake CPU, K80 GPU) multiple features can be provided:

```
$ docker exec -ti $(docker ps -ql) dockerd --debug --platform-feature=test1 --platform-feature=test2
*snip*
DEBU[2019-02-07T10:10:15.628135300Z] platform.features to pick images from ManifestLists: [test1 test2]
*snip*
```

The Matcher now searches for an exact match of the (sorted) features.

```
DEBU[2019-02-07T10:25:07.484567500Z] Calling POST /v1.39/images/create?fromImage=qnib%2Fplain-manifestlist&tag=latest
DEBU[2019-02-07T10:25:07.484677700Z] Create platform spec with Features: [test1 test2]
DEBU[2019-02-07T10:25:07.484890600Z] Trying to pull qnib/plain-manifestlist from https://registry-1.docker.io v2
DEBU[2019-02-07T10:25:09.653208900Z] Pulling ref from V2 registry: qnib/plain-manifestlist:latest
DEBU[2019-02-07T10:25:09.655948100Z] docker.io/qnib/plain-manifestlist:latest resolved to a manifestList object with 4 entries; looking for a linux/amd64/amd64 match
DEBU[2019-02-07T10:25:09.656912100Z] m.Features: [test1 test2] | [test1] platform.Features
DEBU[2019-02-07T10:25:09.659387100Z] m.Features: [test1 test2] | [test2] platform.Features
DEBU[2019-02-07T10:25:09.660330800Z] m.Features: [test1 test2] | [test3] platform.Features
DEBU[2019-02-07T10:25:09.662014600Z] m.Features: [test1 test2] | [test1 test2] platform.Features
DEBU[2019-02-07T10:25:09.662756300Z] found match for linux/amd64 with media type application/vnd.docker.distribution.manifest.v2+json, digest sha256:c76f5fd2572c6343fc04949e16a18530150f892509018fc190b0b7ab0bb775bf
```

