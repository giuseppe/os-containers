# os-containers

A Go implementation of the Almighty [Atomic system containers](http://www.projectatomic.io/blog/2016/09/intro-to-system-containers/).

os-containers allow to run system services as OCI containers.  Once
the container is configured, systemd and the OCI runtime are used for
managing the lifecycle of the containers.

Images are stored in an OSTree repository, the biggest advantage is
that similar files are deduplicated using hard links, so different
versions of a container can be checked out, and if something goes
wrong during the update, it is possible to switch immediately to the
previous working version.

Another important feature is that during the installation it is
possible to copy files to the host,  This enables the configuration,
if the container supports it, through files on the host.

os-containers can also be used as non-root user, in this case
`systemctl --user` is used to manage the container, one limitation in
this case is that the copy of files to the host is disabled.

## Example

```console
# os-containers pull docker://docker.io/gscrivano/etcd
Getting image source signatures
Copying blob sha256:7bf62da3748ffa4e07dafe194786e8674426e4e8ecf835b0fd829438b1b91090
 81.57 MB / 81.57 MB [======================================================] 1s
Copying blob sha256:01086524a4f965dd0c0b7ba07fb0a597e912497695a8cb03d7956dde9064f1f3
 14.48 MB / 14.48 MB [======================================================] 0s
Copying blob sha256:9b94d59f808a765dc2ae8981631af8bef3cbfd9e2b65c80815cfcf0151d6136d
 360 B / 360 B [============================================================] 0s
Copying blob sha256:c064886121cb1c1487b9548ebe62e908629add2d88c573f85093051bbb1dc496
 346 B / 346 B [============================================================] 0s
Copying blob sha256:a671281b3a62e1f2820dac2b2e3dde29afb636681fb1f624312671a48497e738
 246 B / 246 B [============================================================] 0s
Copying blob sha256:19610b56b7361e0f991ce3705d83ba5d44f8f6893e8d2b7643f259ab40b47a43
 2.09 KB / 2.09 KB [========================================================] 0s
Copying blob sha256:8f487db62788e6bee6171460ad7e19a896a1fa08d8a5916cdc0d6790f83768fb
 5.26 MB / 5.26 MB [========================================================] 0s
Copying blob sha256:644d822cfecdd80f649150b6ccfacabedb1df83468c9f7e359f47980ca64e78d
 786 B / 786 B [============================================================] 0s
Copying config sha256:468e8c52d4a6b73e704786e7830df7ef9ad0ac70d9c7de97776976fd1ecc2098
 6.25 KB / 6.25 KB [========================================================] 0s
Writing manifest to image destination
Storing signatures
# os-container install docker.io/gscrivano/etcd
2018/08/05 12:51:46 copied /etc/etcd/etcd.conf
2018/08/05 12:51:46 copied /usr/local/bin/etcdctl
2018/08/05 12:51:46 systemctl daemon-reload
2018/08/05 12:51:46 systemctl enable etcd
```

If you wish you can modify the configuration file:
```console
# emacs -nw /etc/etcd/etcd.conf
```

Now that everything is ready, we can run the service:
```console
# systemctl start etcd
# systemctl status etcd
● etcd.service - Etcd Server
   Loaded: loaded (/etc/systemd/system/etcd.service; enabled; vendor preset: disabled)
   Active: active (running) since Sun 2018-08-05 12:53:58 CEST; 6s ago
 Main PID: 21431 (runc)
    Tasks: 11 (limit: 4915)
   Memory: 6.7M
   CGroup: /system.slice/etcd.service
           └─21431 /usr/bin/runc --systemd-cgroup run etcd
```

Let's imagine we pulled a new version of the container with the
above pull command, we can update the container as:
```console
# os-containers update etcd
2018/08/05 12:55:19 systemctl --now disable etcd
2018/08/05 12:55:19 systemd-tmpfiles --delete /etc/tmpfiles.d/etcd.conf
2018/08/05 12:55:19 file /etc/etcd/etcd.conf deleted
2018/08/05 12:55:19 file /usr/local/bin/etcdctl deleted
2018/08/05 12:55:19 copied /etc/etcd/etcd.conf
2018/08/05 12:55:19 copied /usr/local/bin/etcdctl
2018/08/05 12:55:19 systemctl daemon-reload
2018/08/05 12:55:19 systemctl enable etcd
2018/08/05 12:55:20 systemctl start etcd
```

The previous deployment is still present on the system, if we are not
happy with the update we can go back to it:
```console
# os-containers rollback etcd
2018/08/05 12:56:49 systemctl --now disable etcd
2018/08/05 12:56:49 systemd-tmpfiles --delete /etc/tmpfiles.d/etcd.conf
2018/08/05 12:56:49 file /etc/etcd/etcd.conf deleted
2018/08/05 12:56:49 file /usr/local/bin/etcdctl deleted
2018/08/05 12:56:49 copied /etc/etcd/etcd.conf
2018/08/05 12:56:49 copied /usr/local/bin/etcdctl
2018/08/05 12:56:49 systemctl daemon-reload
2018/08/05 12:56:49 systemctl enable etcd
2018/08/05 12:56:49 systemctl start etcd
```

Once we are done with the container:

```console
# os-container uninstall etcd
2018/08/05 12:58:30 systemctl --now disable etcd
2018/08/05 12:58:30 systemd-tmpfiles --delete /etc/tmpfiles.d/etcd.conf
2018/08/05 12:58:30 file /etc/etcd/etcd.conf deleted
2018/08/05 12:58:31 file /usr/local/bin/etcdctl deleted
```
