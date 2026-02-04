# Firewall Cgroup Requirements for Nested Container Environments

This document describes how the bosh-agent nftables firewall uses cgroups for process identification, and the implications for running the agent in containerized environments.

## Overview

The bosh-agent firewall uses nftables to restrict which processes can access sensitive endpoints (monit, NATS). To identify the agent process (and distinguish it from potentially malicious workloads), the firewall uses **cgroup-based socket matching**.

On cgroup v2 systems (Ubuntu Noble and newer), the firewall creates rules like:

```
socket cgroupv2 level 2 eq <cgroup-inode-id> ip daddr 127.0.0.1 tcp dport 2822 accept
```

This rule matches outgoing TCP packets where:
1. The socket belongs to a process in a specific cgroup (identified by inode ID)
2. The destination is localhost port 2822 (monit)

## How Cgroup ID Resolution Works

When the agent starts, it performs the following steps:

1. **Read cgroup path**: Parse `/proc/self/cgroup` to get the agent's cgroup path
   ```
   # Example output on cgroup v2:
   0::/system.slice/bosh-agent.service
   ```

2. **Resolve cgroup inode**: Look up the inode number of the cgroup directory
   ```go
   fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)
   // e.g., /sys/fs/cgroup/system.slice/bosh-agent.service
   stat, _ := syscall.Stat(fullPath)
   inodeID := stat.Ino
   ```

3. **Create nftables rule**: Use the inode ID in the `socket cgroupv2` match expression

## Container Environment Requirements

For the firewall to work correctly in containers, **the container must have access to the host's cgroup filesystem**. This is typically achieved through a bind mount:

```go
garden.BindMount{
    SrcPath: "/sys/fs/cgroup",
    DstPath: "/sys/fs/cgroup",
    Mode:    garden.BindMountModeRW,
    Origin:  garden.BindMountOriginHost,
}
```

### Why This Is Required

The kernel's nftables `socket cgroupv2` matching works by comparing the socket's cgroup inode (as seen by the kernel) against the inode specified in the rule. For this to work:

1. The inode ID in the rule must match what the kernel sees
2. The kernel always uses the **host's** cgroup hierarchy (cgroups are a kernel-level concept)
3. Therefore, the agent must look up inodes from the **same** filesystem the kernel uses

If the container has its own isolated `/sys/fs/cgroup` (e.g., a separate cgroup namespace without bind mount), then:
- The cgroup path from `/proc/self/cgroup` might be relative to the container's root cgroup
- The inode lookup would return a different inode (or fail entirely)
- The nftables rule would not match the agent's traffic

### Container Runtime Configurations

| Runtime | Configuration | Firewall Works? |
|---------|--------------|-----------------|
| Garden-runc (privileged) | Bind-mounts `/sys/fs/cgroup` from host | ✅ Yes |
| Docker (--privileged) | Typically bind-mounts cgroups | ✅ Yes |
| Docker (unprivileged) | May have isolated cgroup namespace | ⚠️ Depends |
| Kubernetes (privileged pod) | Host cgroup namespace | ✅ Yes |
| Kubernetes (unprivileged pod) | Container cgroup namespace | ❌ No |

## Nested Container Scenarios

### Scenario 1: Garden Container on Host VM

```
Host VM (cgroup v2)
└── /sys/fs/cgroup (host filesystem)
    └── Garden container (bind-mounts /sys/fs/cgroup)
        └── bosh-agent reads /proc/self/cgroup
            → Gets path like /user.slice/container-xxx/agent
            → Looks up /sys/fs/cgroup/user.slice/container-xxx/agent
            → Gets real inode from host filesystem
            → ✅ Firewall rules work
```

### Scenario 2: Nested Garden (L1 → L2)

```
Host VM
└── /sys/fs/cgroup (host filesystem)
    └── L1 Garden container (bind-mounts /sys/fs/cgroup from host)
        └── L1's /sys/fs/cgroup → points to host's cgroup filesystem
            └── L2 Garden container (bind-mounts /sys/fs/cgroup from L1)
                └── L2's /sys/fs/cgroup → still points to host's cgroup filesystem
                    └── bosh-agent in L2
                        → Gets path from /proc/self/cgroup
                        → Looks up inode from host's cgroup filesystem
                        → ✅ Firewall rules work
```

The key insight is that as long as each nesting level bind-mounts `/sys/fs/cgroup` from its parent, the innermost container still sees the **host's cgroup filesystem** and can resolve correct inode IDs.

### Scenario 3: VM Inside Container (bosh-lite)

```
Concourse Worker (host)
└── Task container (bind-mounts /sys/fs/cgroup)
    └── start-bosh.sh creates bosh-lite VM
        └── VM has its OWN cgroup filesystem
            └── bosh-agent in VM
                → Reads /proc/self/cgroup → gets VM-local path
                → Looks up /sys/fs/cgroup/... → uses VM's cgroup filesystem
                → ✅ Firewall rules work (VM cgroups are self-contained)
```

When the agent runs in a **true VM** (not a container), it has its own kernel and cgroup hierarchy. The firewall works because:
- `/proc/self/cgroup` returns paths relative to the VM's cgroup root
- `/sys/fs/cgroup` is the VM's own cgroup filesystem
- Inode lookups resolve against the VM's cgroup hierarchy
- The VM's kernel uses the same cgroup hierarchy for socket matching

## Failure Modes

### 1. Missing Cgroup Bind Mount

If a container doesn't have `/sys/fs/cgroup` bind-mounted from the host:

```
Error: getting cgroup ID for /user.slice/container-xxx: stat /sys/fs/cgroup/user.slice/container-xxx: no such file or directory
```

The agent bootstrap will fail because it cannot resolve the cgroup inode.

### 2. Cgroup Namespace Isolation

If a container has its own cgroup namespace with a different view:

```
# Inside container:
$ cat /proc/self/cgroup
0::/

# The root cgroup "/" exists in the container's view
# but looking up /sys/fs/cgroup/ returns a different inode
# than what the kernel uses for socket matching
```

The agent might create rules, but they won't match traffic because the inode ID is wrong.

### 3. Cgroup v1 vs v2 Mismatch

If the agent detects cgroup v2 but the container only has cgroup v1 controllers mounted:

```
# Container has hybrid cgroup setup
$ cat /proc/self/cgroup
12:net_cls,net_prio:/container-xxx
0::/container-xxx

# Agent detects v2 (0::) but /sys/fs/cgroup might be v1 layout
```

The inode lookup might fail or return unexpected results.

## Recommendations

### For Container Operators

1. **Always bind-mount `/sys/fs/cgroup`** from the host when running bosh-agent in containers
2. **Use privileged containers** if the agent needs to manage firewall rules
3. **Ensure consistent cgroup version** between host and container

### For bosh-agent Development

1. **Current behavior**: Fail fast if cgroup ID lookup fails
2. **Alternative consideration**: Graceful degradation (skip cgroup matching, log warning)
3. **Testing**: The integration tests validate nested container scenarios with proper bind mounts

## Related Files

| File | Description |
|------|-------------|
| `platform/firewall/cgroup_linux.go` | Cgroup detection and ID resolution |
| `platform/firewall/nftables_firewall.go` | Firewall rule creation using cgroup IDs |
| `integration/installerdriver/driver_garden.go` | Container creation with cgroup bind mounts |
| `integration/garden/nested_garden_firewall_test.go` | Tests for nested container scenarios |

## References

- [nftables socket expression](https://wiki.nftables.org/wiki-nftables/index.php/Matching_connection_tracking_stateful_metainformation#socket)
- [Linux cgroup v2 documentation](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
- [containerd cgroups library](https://github.com/containerd/cgroups)
