#!/usr/bin/env bash
# detach_device.sh tears down a single mount directory via the shared detach_mount helper. The Go
# caller prepends "SM=..." and "DIR=..." and concatenates detach_mount.sh in front of this file.
# The empty HUP handler (trap '' HUP) keeps this reparented shell alive to finish the umount/rm even
# if a fuser-kill inside detach_mount takes down the SSH session (see detach_mount.sh).
: "${DIR:?}"
trap '' HUP
detach_mount "$DIR"
